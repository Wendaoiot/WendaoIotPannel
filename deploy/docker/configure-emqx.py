#!/usr/bin/env python3
"""配置 EMQX 5.x 认证/ACL 链（零第三方依赖，仅标准库）。

读取 .env（见 .env.example），登录 Dashboard 后：
  1) 确保内置数据库认证器（bcrypt）在 HTTP 认证器之前；
  2) 在内置库中创建平台超管（后端自身连接 broker 用）；
  3) 创建指向后端 /mqtt/auth、/mqtt/acl 的 HTTP 认证/授权（X-Auth-Key）；
  4) no_match=deny、deny_action=disconnect、关闭授权缓存；
  5) 创建后端互踢用 REST API Key（secret 仅返回一次，写入 emqx_api.env）。

用法： python3 configure-emqx.py [--base http://127.0.0.1:18083]
环境变量优先于 .env 文件。
"""

import argparse
import json
import os
import sys
import time
import urllib.error
import urllib.request

HERE = os.path.dirname(os.path.abspath(__file__))


def load_env_file(path):
    if not os.path.exists(path):
        return
    with open(path, "r", encoding="utf-8") as f:
        for line in f:
            line = line.strip()
            if not line or line.startswith("#") or "=" not in line:
                continue
            k, v = line.split("=", 1)
            os.environ.setdefault(k.strip(), v.strip())


def env(key, default=""):
    return os.environ.get(key, default)


def call(base, method, path, token=None, body=None):
    url = base + path
    data = None
    headers = {"Content-Type": "application/json"}
    if token:
        headers["Authorization"] = "Bearer " + token
    if body is not None:
        data = json.dumps(body).encode("utf-8")
    req = urllib.request.Request(url, data=data, headers=headers, method=method)
    try:
        with urllib.request.urlopen(req, timeout=10) as resp:
            raw = resp.read().decode("utf-8")
            return True, resp.status, json.loads(raw) if raw else None
    except urllib.error.HTTPError as e:
        raw = e.read().decode("utf-8", "replace")
        try:
            parsed = json.loads(raw) if raw else None
        except json.JSONDecodeError:
            parsed = raw
        return False, e.code, parsed
    except urllib.error.URLError:
        return False, -1, None


def log(fmt, *args):
    print(fmt % args if args else fmt)


def main():
    parser = argparse.ArgumentParser(description="Configure EMQX auth/ACL chain")
    parser.add_argument("--base", default=None, help="EMQX Dashboard base URL")
    parser.add_argument("--env-file", default=None, help="extra env file to load")
    parser.add_argument("--skip-default-env", action="store_true",
                        help="do not load deploy/docker/.env (bare-metal deployments)")
    parser.add_argument("--callback-host", default=None,
                        help="host EMQX uses to reach the backend (default: host.docker.internal for Docker, 127.0.0.1 for bare-metal)")
    parser.add_argument("--out", default=None, help="path for emqx_api.env output")
    args = parser.parse_args()

    if not args.skip_default_env:
        load_env_file(os.path.join(HERE, ".env"))
    if args.env_file:
        load_env_file(args.env_file)

    dash_port = env("EMQX_DASHBOARD_PORT", "18083")
    base = args.base or env("EMQX_DASHBOARD", "http://127.0.0.1:" + dash_port)
    admin_user = env("EMQX_DASHBOARD_USER", "admin")
    admin_pass = env("EMQX_DASHBOARD_PASSWORD")
    srv_user = env("MQTT_SERVER_USER", "wendao_server")
    srv_pass = env("MQTT_SERVER_PASSWORD")
    hook_secret = env("MQTT_HOOK_SECRET")
    api_key_name = env("EMQX_API_KEY_NAME", "wendao-backend-kick")
    server_port = env("SERVER_PORT", "8080")
    callback_host = args.callback_host or env("BACKEND_CALLBACK_HOST", "host.docker.internal")
    url_auth = "http://%s:%s/api/v1/mqtt/auth" % (callback_host, server_port)
    url_acl = "http://%s:%s/api/v1/mqtt/acl" % (callback_host, server_port)
    out_path = args.out or os.path.join(HERE, "emqx_api.env")

    missing = [n for n, v in (
        ("EMQX_DASHBOARD_PASSWORD", admin_pass),
        ("MQTT_HOOK_SECRET", hook_secret),
        ("MQTT_SERVER_PASSWORD", srv_pass),
    ) if not v or v.startswith("change_me")]
    if missing:
        log("FATAL: 请先在 .env 中设置（不能保留 change_me 占位值）: %s", ", ".join(missing))
        return 1

    # 0. 等待 Dashboard 就绪并登录
    token = None
    for _ in range(40):
        ok, _, data = call(base, "POST", "/api/v5/login",
                           body={"username": admin_user, "password": admin_pass})
        if ok and isinstance(data, dict) and data.get("token"):
            token = data["token"]
            break
        time.sleep(3)
    if not token:
        log("FATAL: 无法登录 EMQX Dashboard（未就绪或凭据错误）")
        return 1
    log("login ok")

    # 1. 认证链现状
    ok, _, data = call(base, "GET", "/api/v5/authentication", token=token)
    chain = data if ok and isinstance(data, list) else []
    ids = [a.get("id") for a in chain]
    log("auth chain: %s", " | ".join(ids))

    # 2. 确保内置数据库认证器（bcrypt）
    builtin_id = "password_based:built_in_database"
    if builtin_id not in ids:
        body = {
            "mechanism": "password_based",
            "backend": "built_in_database",
            "user_id_type": "username",
            "password_hash_algorithm": {"name": "bcrypt"},
            "enable": True,
        }
        ok, st, _ = call(base, "POST", "/api/v5/authentication", token=token, body=body)
        if not ok:
            log("FATAL: 创建内置数据库认证器失败 (status=%s)", st)
            return 1
    else:
        log("built_in_database authenticator exists")

    # 3. 平台超管写入内置库
    users_path = "/api/v5/authentication/password_based:built_in_database/users"
    ok, st, _ = call(base, "POST", users_path, token=token,
                     body={"user_id": srv_user, "password": srv_pass, "is_superuser": True})
    if not ok:
        if st in (400, 409):
            ok2, st2, _ = call(base, "PUT", users_path + "/" + srv_user, token=token,
                               body={"password": srv_pass, "is_superuser": True})
            if not ok2:
                log("FATAL: 更新平台超管失败 (status=%s)", st2)
                return 1
        else:
            log("FATAL: 创建平台超管失败 (status=%s)", st)
            return 1

    # 4. 保证内置认证在 HTTP 之前（顺序影响命中）
    ok, _, data = call(base, "GET", "/api/v5/authentication", token=token)
    chain = data if ok and isinstance(data, list) else []
    ids = [a.get("id") for a in chain]
    http_id = "password_based:http"
    need_http = http_id not in ids
    if http_id in ids and builtin_id in ids and ids.index(http_id) < ids.index(builtin_id):
        call(base, "DELETE", "/api/v5/authentication/" + http_id, token=token)
        need_http = True
    if need_http:
        body = {
            "mechanism": "password_based",
            "backend": "http",
            "method": "post",
            "url": url_auth,
            "headers": {"X-Auth-Key": hook_secret},
            "body": {"clientid": "${clientid}", "username": "${username}",
                     "password": "${password}"},
            "request_timeout": "5s",
            "connect_timeout": "5s",
        }
        ok, st, _ = call(base, "POST", "/api/v5/authentication", token=token, body=body)
        if not ok:
            log("FATAL: 创建 HTTP 认证器失败 (status=%s)", st)
            return 1

    ok, _, data = call(base, "GET", "/api/v5/authentication", token=token)
    for a in (data if ok else []):
        log("  authenticator: %s (enable=%s)", a.get("id"), a.get("enable"))

    # 5. 授权全局设置（fail-closed）
    call(base, "PUT", "/api/v5/authorization/settings", token=token,
         body={"no_match": "deny", "deny_action": "disconnect",
               "cache": {"enable": False}})

    # 6. HTTP 授权源
    ok, _, data = call(base, "GET", "/api/v5/authorization/sources", token=token)
    types = [s.get("type") for s in data] if ok and isinstance(data, list) else []
    if "http" not in types:
        body = {
            "type": "http",
            "enable": True,
            "method": "post",
            "url": url_acl,
            "headers": {"X-Auth-Key": hook_secret},
            "body": {"username": "${username}", "clientid": "${clientid}",
                     "topic": "${topic}", "action": "${action}"},
            "request_timeout": "5s",
            "connect_timeout": "5s",
        }
        ok, st, err = call(base, "POST", "/api/v5/authorization/sources", token=token, body=body)
        if not ok and not (isinstance(err, dict)
                           and "duplicated_authz_source_type" in json.dumps(err)):
            log("FATAL: 创建 HTTP 授权源失败 (status=%s)", st)
            return 1
    else:
        log("http authorization source exists")

    ok, _, data = call(base, "GET", "/api/v5/authorization/sources", token=token)
    for s in (data if ok else []):
        log("  authorization source: %s (enable=%s)", s.get("type"), s.get("enable"))

    # 7. REST API Key（互踢用）：已存在则轮换
    keys_path = "/api/v5/api_key"
    key_body = {
        "enable": True,
        "name": api_key_name,
        "expired_at": "2059-12-31T00:00:00Z",
        "desc": "Backend: kick duplicate device sessions",
    }
    ok, st, data = call(base, "POST", keys_path, token=token, body=key_body)
    if not ok:
        call(base, "DELETE", keys_path + "/" + api_key_name, token=token)
        ok, st, data = call(base, "POST", keys_path, token=token, body=key_body)
    if not ok or not isinstance(data, dict) or not data.get("api_key"):
        log("WARN: API Key 自动创建失败 (status=%s)，请在 Dashboard → 访问控制 → API 密钥手工创建", st)
    else:
        api_key = data["api_key"]
        api_secret = data.get("api_secret", "")
        log("API Key ready: api_key=%s", api_key)
        os.makedirs(os.path.dirname(os.path.abspath(out_path)), exist_ok=True)
        with open(out_path, "w", encoding="utf-8") as f:
            f.write("WQ_EMQX_API_KEY=%s\nWQ_EMQX_API_SECRET=%s\n" % (api_key, api_secret))
        log("secret 仅本次返回，已写入 %s（同步到 server/config.yaml 的 emqx 段或作为环境变量）", out_path)

    log("=== DONE ===")
    return 0


if __name__ == "__main__":
    sys.exit(main())
