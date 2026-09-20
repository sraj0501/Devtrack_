"""Windows acceptance against real Managed data. Never inserts queue fixtures.

The temporary source admin server shares the configured PostgreSQL database.
Browser requests are restricted to that server. Artifacts are private/ignored.
"""
from __future__ import annotations

import argparse
import json
import os
from pathlib import Path
import socket
import subprocess
import sys
import time
import traceback
from urllib.parse import urlparse
from urllib.request import urlopen
import uuid

from dotenv import dotenv_values
from playwright.sync_api import Error as PlaywrightError, expect, sync_playwright

ROOT = Path(__file__).resolve().parents[1]


def hold_prototype(context, browser, page, base, run, action_id, server_pid):
    """Hand real persisted results to the user; closing tabs is normal, not a failed test."""
    links = {'dashboard': base + '/admin/',
             'reviewed_action': base + f'/admin/queue/{action_id}', 'eod': page.url}
    for name in ('dashboard', 'reviewed_action'):
        context.new_page().goto(links[name])
    page.bring_to_front()
    state = {'status': 'ready', 'links': links, 'server_pid': server_pid}
    (run / 'prototype.json').write_text(json.dumps(state, indent=2), encoding='utf-8')
    print(f'PROTOTYPE READY: {links["eod"]}', flush=True)
    print('The browser is yours. Close its window to stop the temporary admin server.', flush=True)
    while browser.is_connected() and context.pages:
        active_page = context.pages[0]
        try:
            active_page.wait_for_timeout(1000)
        except PlaywrightError:
            if active_page.is_closed() and browser.is_connected() and context.pages:
                continue
            break
    state['status'] = 'closed'
    (run / 'prototype.json').write_text(json.dumps(state, indent=2), encoding='utf-8')


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument('--env-file', type=Path, required=True)
    parser.add_argument('--stage-timeout', type=int, default=300)
    parser.add_argument('--showcase', action='store_true',
                        help='Show the live browser and keep the prototype open until its window is closed.')
    parser.add_argument('--inspect-result', type=Path,
                        help='Reopen real actions from a previous passing result.json without rerunning the demo.')
    args = parser.parse_args()
    if args.inspect_result and not args.showcase:
        parser.error('--inspect-result requires --showcase')
    if sys.platform != 'win32':
        parser.error('This runner currently supports native Windows only.')
    if not args.env_file.is_file():
        parser.error('The specified Managed environment file does not exist.')
    env = dict(os.environ)
    env.update({k: v for k, v in dotenv_values(args.env_file).items() if v is not None})
    env['DEVTRACK_ENV_FILE'] = str(args.env_file.resolve())
    # Give this run's real LLM processing the same budget as staging observation.
    env['HTTP_TIMEOUT_LONG'] = str(max(int(env.get('HTTP_TIMEOUT_LONG', '60')), args.stage_timeout))
    # The installed client's EOD route uses its standard HTTP client.
    env['HTTP_TIMEOUT'] = str(max(int(env.get('HTTP_TIMEOUT', '30')), args.stage_timeout))
    for key in ('POSTGRES_URL', 'ADMIN_USERNAME', 'ADMIN_PASSWORD', 'ADMIN_SECRET_KEY'):
        if not env.get(key):
            parser.error(f'Managed environment is missing {key}.')
    # Browser credentials may differ from an ADMIN_PASSWORD stored as a hash.
    password = os.environ.get('DEVTRACK_E2E_ADMIN_PASSWORD', env['ADMIN_PASSWORD'])
    if password.startswith(('scrypt$', '$2a$', '$2b$')):
        parser.error('Set DEVTRACK_E2E_ADMIN_PASSWORD to the login password for this run.')
    run = ROOT / '.codex-cache' / ('admin-e2e-' + uuid.uuid4().hex)
    run.mkdir(parents=True)
    print(f'Private artifacts: {run}', flush=True)
    with socket.socket() as sock:
        sock.bind(('127.0.0.1', 0))
        port = sock.getsockname()[1]
    base = f'http://127.0.0.1:{port}'
    server_env = dict(env, PYTHONUTF8='1')
    server_env['PYTHONPATH'] = str(ROOT / 'devtrack_server')
    server_env['ADMIN_PORT'] = str(port)
    server = None
    try:
        with (run / 'admin.log').open('w', encoding='utf-8') as log:
            server = subprocess.Popen(
                [sys.executable, '-m', 'uvicorn', 'backend.admin.app:app',
                 '--host', '127.0.0.1', '--port', str(port)],
                cwd=ROOT / 'devtrack_server', env=server_env,
                stdout=log, stderr=subprocess.STDOUT, creationflags=subprocess.CREATE_NO_WINDOW,
            )
            for _ in range(60):
                if server.poll() is not None:
                    raise RuntimeError('Source admin server exited; inspect admin.log.')
                try:
                    with urlopen(base + '/admin/login', timeout=2) as response:
                        if response.status == 200:
                            break
                except OSError:
                    time.sleep(1)
            else:
                raise RuntimeError('Admin server readiness timed out.')

            with sync_playwright() as pw:
                browser = pw.chromium.launch(headless=not args.showcase,
                                             slow_mo=150 if args.showcase else 0)
                viewport_options = ({'no_viewport': True} if args.showcase else
                                    {'viewport': {'width': 1440, 'height': 1000}})
                context = browser.new_context(**viewport_options, accept_downloads=False)
                # No CDN requests, downloads, external navigation, or saved login sessions.
                context.route('**/*', lambda route: route.continue_()
                              if urlparse(route.request.url).netloc == urlparse(base).netloc
                              else route.abort())
                page = context.new_page()
                tracing = False
                demo = None
                demo_log = None
                try:
                    page.goto(base + '/admin/login')
                    page.locator('#username').fill(env['ADMIN_USERNAME'])
                    page.locator('#password').fill(password)
                    page.locator('button[type=submit]').click()
                    expect(page).to_have_url(base + '/admin/')
                    expect(page.locator('.topbar-title')).to_have_text('Dashboard')
                    if args.inspect_result:
                        evidence = json.loads(args.inspect_result.read_text(encoding='utf-8'))
                        if evidence.get('result') != 'passed':
                            raise ValueError('Only a passing demo result can be reopened.')
                        action_id = int(evidence['rejected_action_id'])
                        page.goto(base + f'/admin/queue/{action_id}')
                        expect(page.locator('[data-field=status]')).to_have_text('rejected')
                        payload = json.loads(page.locator('[data-field=payload]').inner_text())
                        assert payload['commit_info']['hash'].startswith(evidence['commit'])
                        page.goto(base + f"/admin/queue/{int(evidence['eod_action_id'])}")
                        expect(page.locator('[data-field=action_type]')).to_have_text('eod_report')
                        hold_prototype(context, browser, page, base, run, action_id, server.pid)
                        return
                    # Login is excluded from tracing so form passwords are not recorded.
                    context.tracing.start(screenshots=True, snapshots=True)
                    tracing = True
                    page.screenshot(path=run / 'dashboard.png', full_page=True)
                    page.goto(base + '/admin/queue')
                    expect(page.locator('.topbar-title')).to_have_text('Pending actions')

                    demo_log = (run / 'demo.log').open('w', encoding='utf-8')
                    demo = subprocess.Popen(
                            ['powershell.exe', '-NoProfile', '-ExecutionPolicy', 'Bypass',
                             '-File', str(ROOT / 'scripts/demo.ps1'), '-Mode', 'Record', '-Automated',
                             '-StageTimeoutSeconds', str(args.stage_timeout),
                             '-EvidencePath', str(run / 'evidence.json')],
                            cwd=ROOT, env=env, stdout=demo_log, stderr=subprocess.STDOUT,
                            creationflags=subprocess.CREATE_NO_WINDOW,
                    )
                    deadline = time.monotonic() + args.stage_timeout + 180
                    evidence = None
                    while time.monotonic() < deadline:
                        if demo.poll() is not None and demo.returncode:
                            raise RuntimeError('Real Managed demo failed; inspect demo.log.')
                        try:
                            evidence = json.loads((run / 'evidence.json').read_text(encoding='utf-8-sig'))
                        except (OSError, ValueError):
                            page.wait_for_timeout(1000)
                            continue
                        page.reload()
                        row = page.locator('tbody tr').filter(has_text=evidence['workspace']).filter(
                            has_text='post_comment').filter(has_text=evidence['ticket'])
                        if row.count() == 1:
                            break
                        page.wait_for_timeout(1000)
                    else:
                        raise RuntimeError('No unique pending commit action appeared before the deadline.')
                    action_id = int(row.get_attribute('data-action-id'))
                    row.get_by_role('link', name=f'Review #{action_id}', exact=True).click()
                    expect(page.locator('[data-field=target]')).to_have_text(evidence['ticket'])
                    expect(page.locator('[data-field=workspace]')).to_have_text(evidence['workspace'])
                    expect(page.locator('[data-field=status]')).to_have_text('pending')
                    confidence = float(page.locator('[data-field=confidence]').inner_text())
                    assert 0 <= confidence <= 1, 'Invalid action confidence'
                    payload = json.loads(page.locator('[data-field=payload]').inner_text())
                    assert payload, 'Empty staged payload'
                    assert payload['commit_info']['hash'].startswith(evidence['commit'])
                    page.screenshot(path=run / 'action.png', full_page=True)
                    if args.showcase:
                        page.wait_for_timeout(4000)
                    page.get_by_role('button', name='Reject action', exact=True).click()
                    expect(page.locator('[data-field=status]')).to_have_text('rejected')
                    expect(page.locator('[data-field=acted_by]')).to_have_text(env['ADMIN_USERNAME'])
                    assert page.locator('[data-field=acted_at]').inner_text() != '—'
                    expect(page.get_by_role('button', name='Reject action', exact=True)).to_have_count(0)
                    page.screenshot(path=run / 'rejected.png', full_page=True)
                    page.goto(base + '/admin/audit')
                    audit = page.locator('tbody tr').filter(has_text='queue_reject').filter(
                        has=page.get_by_text(f'action_id={action_id}', exact=True))
                    expect(audit).to_have_count(1)
                    print(f'PASS: commit action #{action_id} rejected and audited; waiting for EOD.', flush=True)
                    if demo.wait(timeout=args.stage_timeout + 180):
                        raise RuntimeError('Real Managed demo failed; inspect demo.log.')
                    evidence = json.loads((run / 'evidence.json').read_text(encoding='utf-8-sig'))
                    page.goto(base + f"/admin/queue/{evidence['eod_action_id']}")
                    expect(page.locator('[data-field=action_type]')).to_have_text('eod_report')
                    expect(page.locator('[data-field=status]')).to_have_text('pending')
                    assert evidence['ticket'] in page.locator('[data-field=payload]').inner_text()
                    page.screenshot(path=run / 'eod.png', full_page=True)
                    evidence.update(result='passed', rejected_action_id=action_id)
                    (run / 'result.json').write_text(json.dumps(evidence, indent=2), encoding='utf-8')
                    print('PASS: real commit review, rejection, audit, and EOD visibility.', flush=True)
                    if args.showcase:
                        # Finish recording before handing the browser to the user.
                        context.tracing.stop(path=run / 'trace.zip')
                        tracing = False
                        hold_prototype(context, browser, page, base, run, action_id, server.pid)
                except Exception:
                    (run / 'evidence.json.cancel').touch()
                    (run / 'failure.txt').write_text(traceback.format_exc(), encoding='utf-8')
                    with (run / 'doctor.log').open('w', encoding='utf-8') as diagnostic:
                        try:
                            subprocess.run(['devtrack', 'doctor'], env=env, stdout=diagnostic,
                                           stderr=subprocess.STDOUT, timeout=30,
                                           creationflags=subprocess.CREATE_NO_WINDOW)
                        except (OSError, subprocess.TimeoutExpired):
                            diagnostic.write('\nCould not collect doctor output.\n')
                    # Never capture a failed login/password form.
                    if tracing:
                        page.screenshot(path=run / 'failure.png', full_page=True)
                        (run / 'failure.html').write_text(page.content(), encoding='utf-8')
                    raise
                finally:
                    if demo is not None and demo.poll() is None:
                        # Let the demo's own finally remove its workspace and restore the daemon.
                        demo.wait(timeout=args.stage_timeout + 180)
                    if demo_log is not None:
                        demo_log.close()
                    if tracing:
                        context.tracing.stop(path=run / 'trace.zip')
                    context.close()
                    browser.close()
    finally:
        if server is not None and server.poll() is None:
            server.terminate()
            try:
                server.wait(timeout=15)
            except subprocess.TimeoutExpired:
                server.kill()
                server.wait(timeout=5)


if __name__ == '__main__':
    try:
        main()
    except Exception as error:
        # Browser errors can include private payload excerpts; keep them local.
        print(f'FAIL ({type(error).__name__}): inspect the private artifacts for details.', file=sys.stderr)
        sys.exit(1)
