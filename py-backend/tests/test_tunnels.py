"""隧道测试：构造与参数校验（mock sshtunnel / pywinrm，不真连）。"""
from pathlib import Path

import pytest

from drivers import ssh_tunnel as ssh_mod
from drivers import winrm_tunnel as winrm_mod
from drivers.ssh_tunnel import SSHTunnel
from drivers.winrm_tunnel import WinRMTunnel


# ---------------- SSH 隧道 ----------------

class FakeForwarder:
    """sshtunnel.SSHTunnelForwarder 的替身。"""

    instances: list["FakeForwarder"] = []

    def __init__(self, address, **kwargs):
        self.address = address
        self.kwargs = kwargs
        self.local_bind_port = 15432
        self.started = False
        self.stopped = False
        FakeForwarder.instances.append(self)

    def start(self):
        self.started = True

    def stop(self):
        self.stopped = True


def test_ssh_tunnel_start_stop(monkeypatch):
    FakeForwarder.instances.clear()
    monkeypatch.setattr(ssh_mod.sshtunnel, "SSHTunnelForwarder", FakeForwarder)

    t = SSHTunnel(host="10.0.0.5", ssh_port=22, user="ops", password="secret",
                  db_host="127.0.0.1", db_port=3306)
    host, port = t.start()
    assert (host, port) == ("127.0.0.1", 15432)

    fw = FakeForwarder.instances[-1]
    assert fw.address == ("10.0.0.5", 22)
    assert fw.kwargs["ssh_username"] == "ops"
    assert fw.kwargs["ssh_password"] == "secret"
    assert fw.kwargs["remote_bind_address"] == ("127.0.0.1", 3306)
    assert fw.kwargs["allow_agent"] is False
    assert fw.kwargs["host_pkey_direct_keys"] is False

    t.stop()
    assert fw.stopped is True


def test_ssh_tunnel_private_key_content(monkeypatch):
    """私钥为内容时写临时文件，stop 后清理。"""
    FakeForwarder.instances.clear()
    monkeypatch.setattr(ssh_mod.sshtunnel, "SSHTunnelForwarder", FakeForwarder)

    pem = ("-----BEGIN OPENSSH PRIVATE KEY-----\n"
           "abc123\n"
           "-----END OPENSSH PRIVATE KEY-----")
    t = SSHTunnel(host="10.0.0.5", user="ops", private_key=pem,
                  key_passphrase="pp", db_host="db.internal", db_port=5432)
    t.start()
    fw = FakeForwarder.instances[-1]
    pkey_path = Path(fw.kwargs["ssh_pkey"])
    assert pkey_path.is_file()
    assert "BEGIN OPENSSH" in pkey_path.read_text(encoding="utf-8")
    assert fw.kwargs["ssh_private_key_password"] == "pp"

    t.stop()
    assert not pkey_path.exists()


def test_ssh_tunnel_private_key_path(tmp_path, monkeypatch):
    """私钥为文件路径时直接使用。"""
    FakeForwarder.instances.clear()
    monkeypatch.setattr(ssh_mod.sshtunnel, "SSHTunnelForwarder", FakeForwarder)

    key_file = tmp_path / "id_rsa"
    key_file.write_text("dummy-key-content\n", encoding="utf-8")
    t = SSHTunnel(host="h", user="u", private_key=str(key_file), db_host="d", db_port=1)
    t.start()
    fw = FakeForwarder.instances[-1]
    assert fw.kwargs["ssh_pkey"] == str(key_file)
    t.stop()


def test_ssh_tunnel_validation():
    # 缺主机
    with pytest.raises(ValueError):
        SSHTunnel(host="", user="u", password="p", db_host="d", db_port=1)
    # 缺用户名
    with pytest.raises(ValueError):
        SSHTunnel(host="h", user="", password="p", db_host="d", db_port=1)
    # 密码与私钥都缺
    with pytest.raises(ValueError):
        SSHTunnel(host="h", user="u", password="", private_key="", db_host="d", db_port=1)
    # 数据库目标无效
    with pytest.raises(ValueError):
        SSHTunnel(host="h", user="u", password="p", db_host="", db_port=0)


def test_ssh_tunnel_missing_key_file(tmp_path):
    t = SSHTunnel(host="h", user="u", private_key=str(tmp_path / "no-such-key"), db_host="d", db_port=1)
    with pytest.raises(ValueError):
        t.start()


# ---------------- WinRM 隧道 ----------------

class FakeResp:
    def __init__(self, code=0, out=b"", err=b""):
        self.status_code = code
        self.std_out = out
        self.std_err = err


class FakeSession:
    def __init__(self, **kwargs):
        self.kwargs = kwargs
        self.commands: list[str] = []
        self.fail_portproxy_add = False

    def run_cmd(self, cmd: str):
        self.commands.append(cmd)
        if self.fail_portproxy_add and "portproxy add" in cmd:
            return FakeResp(5, b"", b"Access is denied.")
        return FakeResp(0, b"")


def test_winrm_tunnel_flow(monkeypatch):
    monkeypatch.setattr(winrm_mod.random, "randint", lambda a, b: 31555)
    sessions: list[FakeSession] = []

    def fake_session_cls(**kw):
        s = FakeSession(**kw)
        sessions.append(s)
        return s

    monkeypatch.setattr(winrm_mod.winrm, "Session", fake_session_cls)

    t = WinRMTunnel(host="10.0.0.9", port=5985, user="Administrator", password="pw",
                    auth_scheme="ntlm", transport="http", db_host="127.0.0.1", db_port=1433)
    host, port = t.start()
    assert (host, port) == ("10.0.0.9", 31555)

    s = sessions[-1]
    assert s.kwargs["endpoint"] == "http://10.0.0.9:5985/wsman"
    assert s.kwargs["auth"] == ("Administrator", "pw")
    assert s.kwargs["transport"] == "ntlm"
    assert s.kwargs["server_cert_validation"] == "ignore"

    joined = "\n".join(s.commands)
    assert "netstat -ano" in joined
    assert "portproxy add v4tov4" in joined
    assert "listenport=31555" in joined
    assert "connectport=1433" in joined
    assert 'advfirewall firewall add rule name="dbawork-31555"' in joined

    t.stop()
    joined2 = "\n".join(s.commands)
    assert "portproxy delete" in joined2
    assert 'advfirewall firewall delete rule name="dbawork-31555"' in joined2


def test_winrm_portproxy_failure_message(monkeypatch):
    monkeypatch.setattr(winrm_mod.random, "randint", lambda a, b: 32000)

    def fake_session_cls(**kw):
        s = FakeSession(**kw)
        s.fail_portproxy_add = True
        return s

    monkeypatch.setattr(winrm_mod.winrm, "Session", fake_session_cls)

    t = WinRMTunnel(host="h", user="u", password="p", db_host="d", db_port=1)
    with pytest.raises(RuntimeError) as ei:
        t.start()
    msg = str(ei.value)
    assert "端口转发建立失败" in msg
    assert "管理员权限" in msg or "WinRM" in msg


def test_winrm_no_free_port(monkeypatch):
    """netstat 始终显示端口被占用 -> 重试后报错。"""
    monkeypatch.setattr(winrm_mod.random, "randint", lambda a, b: 31000)

    def fake_session_cls(**kw):
        s = FakeSession(**kw)
        s.run_cmd = lambda cmd: FakeResp(
            0, b"  TCP    0.0.0.0:31000    0.0.0.0:0    LISTENING    4\r\n"
        )
        return s

    monkeypatch.setattr(winrm_mod.winrm, "Session", fake_session_cls)

    t = WinRMTunnel(host="h", user="u", password="p", db_host="d", db_port=1)
    with pytest.raises(RuntimeError) as ei:
        t.start()
    assert "空闲端口" in str(ei.value)


def test_winrm_validation():
    # 缺主机
    with pytest.raises(ValueError):
        WinRMTunnel(host="", user="u", password="p", db_host="d", db_port=1)
    # 缺用户名
    with pytest.raises(ValueError):
        WinRMTunnel(host="h", user="", password="p", db_host="d", db_port=1)
    # 缺密码
    with pytest.raises(ValueError):
        WinRMTunnel(host="h", user="u", password="", db_host="d", db_port=1)
    # 非法认证方式
    with pytest.raises(ValueError):
        WinRMTunnel(host="h", user="u", password="p", auth_scheme="noauth", db_host="d", db_port=1)
    # 非法传输协议
    with pytest.raises(ValueError):
        WinRMTunnel(host="h", user="u", password="p", transport="tcp", db_host="d", db_port=1)
