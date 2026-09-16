#!/usr/bin/env python3
"""Exercise real Server/Agent images on a private Docker network.

All state is temporary and retained; containers are stopped, never removed.
"""
import http.cookiejar
import json
import os
from pathlib import Path
import secrets
import shutil
import subprocess
import sys
import tempfile
import time
import urllib.error
import urllib.request

image, version = sys.argv[1:3]
agent_lock = json.loads(Path('build/agent.json').read_text())
agent_image = os.environ.get('KOMARI_SMOKE_AGENT_IMAGE', agent_lock['image'])
root = Path(tempfile.mkdtemp(prefix='komari-smoke-'))
data = root/'data'
data.mkdir()
network = 'komari-smoke-' + secrets.token_hex(5)
containers = []
password = 'Aa1-' + secrets.token_urlsafe(30)
node_token = ''
base = ''
opener = urllib.request.build_opener(urllib.request.HTTPCookieProcessor(http.cookiejar.CookieJar()))

def docker(*args):
    process = subprocess.run(['docker', *args], text=True, stdout=subprocess.PIPE, stderr=subprocess.PIPE)
    if process.returncode:
        # Container start arguments can contain synthetic credentials; never print argv.
        raise RuntimeError(process.stderr.strip())
    return process.stdout.strip()

def request(path, payload=None):
    raw = None if payload is None else json.dumps(payload).encode()
    req = urllib.request.Request(base+path, data=raw, headers={'Content-Type': 'application/json', 'Origin': base})
    try:
        with opener.open(req, timeout=5) as response:
            return json.load(response)
    except urllib.error.HTTPError as error:
        raise RuntimeError(path + ': HTTP ' + str(error.code) + ' ' + error.read().decode(errors='replace')) from error

def rpc(method, params=None):
    result = request('/api/rpc2', {'jsonrpc': '2.0', 'id': 1, 'method': method, 'params': params or {}})
    if 'error' in result:
        raise RuntimeError(method + ': ' + str(result['error']))
    return result['result']

def wait_for(check, description, timeout=90):
    last_error = None
    deadline = time.monotonic() + timeout
    while time.monotonic() < deadline:
        try:
            value = check()
            if value:
                return value
        except (OSError, ValueError, RuntimeError, KeyError) as error:
            last_error = error
        time.sleep(1)
    raise RuntimeError('Timed out: '+description+'; '+str(last_error))

def start_server(suffix):
    global base
    name = network+'-'+suffix
    docker('run', '-d', '--name', name, '--network', network, '--network-alias', 'server',
           '-p', '127.0.0.1::25774', '--mount', 'type=bind,src='+str(data)+',dst=/app/data', image)
    containers.append(name)
    port = json.loads(docker('inspect', name))[0]['NetworkSettings']['Ports']['25774/tcp'][0]['HostPort']
    base = 'http://127.0.0.1:'+port
    return name

def login():
    response = request('/api/login', {'username': 'smoke-admin', 'password': password})
    return response.get('status') == 'success'

try:
    docker('network', 'create', network)
    first_server = start_server('server-1')
    wait_for(lambda: request('/api/install/status')['data']['required'], 'fresh install page')
    response = request('/api/install/complete', {
        'username': 'smoke-admin', 'password': password,
        'sitename': 'Fork smoke test', 'description': 'Disposable test data', 'metric_dsn': './data/metrics.db'})
    assert response['status'] == 'success', 'installation failed'
    wait_for(login, 'login after installation')
    assert rpc('public:getVersion')['version'] == version, 'embedded Server version mismatch'
    with opener.open(base+'/admin', timeout=5) as response:
        assert b'<html' in response.read().lower(), 'frontend HTML missing'
    client = rpc('admin:addClient', {'name': 'smoke-agent'})
    node_token = client['token']
    uuid = client['uuid']
    agent = network+'-agent'
    docker('run', '-d', '--name', agent, '--network', network,
           '-e', 'AGENT_ENDPOINT=http://server:25774', '-e', 'AGENT_TOKEN='+node_token,
           '-e', 'AGENT_RECONNECT_INTERVAL=1', '-e', 'AGENT_INTERVAL=1',
           '-e', 'AGENT_DISABLE_AUTO_UPDATE=false', agent_image, '--disable-web-ssh=false')
    containers.append(agent)
    def online():
        status = rpc('common:getNodesLatestStatus', {'uuid': uuid})
        return status if status and status.get('online') and status.get('ram_total', 0) > 0 else None
    status = wait_for(online, 'Agent connects and reports metrics')
    assert status['disk_total'] > 0, 'disk metrics missing'
    task = rpc('admin:exec', {'clients': [uuid], 'command': 'printf komari-smoke-ok'})
    def task_done():
        result = rpc('admin:getTaskById', {'task_id': task['task_id']})
        return any(x.get('exit_code') == 0 and 'komari-smoke-ok' in x.get('result', '') for x in result['results'])
    wait_for(task_done, 'remote command result')
    docker('restart', agent)
    previous_time = status['time']
    wait_for(lambda: (online() or {}).get('time', previous_time) != previous_time, 'fresh reports after Agent restart')
    docker('stop', agent)
    docker('stop', first_server)
    shutil.copytree(data, root/'stopped-data-backup')
    start_server('server-2')
    wait_for(login, 'login with persisted account after container replacement')
    assert rpc('admin:getClient', {'uuid': uuid})['uuid'] == uuid, 'node did not persist'
    docker('start', agent)
    wait_for(online, 'Agent reconnects after Server replacement')
    print('PASS: install, login, frontend, versions, metrics, remote task, Agent restart, Server recreation and persisted node.')
    print('Retained test data and stopped-data backup: '+str(root))
except Exception as error:
    message = str(error).replace(password, '[redacted]')
    if node_token:
        message = message.replace(node_token, '[redacted]')
    print('FAIL: '+message, file=sys.stderr)
    for container in containers:
        result = subprocess.run(['docker', 'logs', '--tail=40', container], text=True, capture_output=True)
        logs = (result.stdout+result.stderr).replace(password, '[redacted]')
        if node_token:
            logs = logs.replace(node_token, '[redacted]')
        print(container+'\n'+logs, file=sys.stderr)
    raise SystemExit(1)
finally:
    for container in reversed(containers):
        subprocess.run(['docker', 'stop', container], stdout=subprocess.DEVNULL, stderr=subprocess.DEVNULL)
