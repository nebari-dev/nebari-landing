#!/usr/bin/env python3
"""
Automated tmate SSH connection and debugging for CI runner.

This script:
1. Monitors GitHub Actions for tmate session
2. Extracts SSH connection details
3. Connects via SSH using paramiko
4. Runs verification commands
5. Reports findings

Usage:
    python3 scripts/tmate-debug.py --run-id 25512392464 --repo nebari-dev/nebari-landing

Requirements:
    pip install paramiko requests
"""

import argparse
import re
import subprocess
import sys
import time
from typing import Optional, Tuple

try:
    import paramiko
    import requests
except ImportError:
    print("ERROR: Required packages not installed")
    print("Run: pip install paramiko requests")
    sys.exit(1)


class TmateDebugger:
    def __init__(self, run_id: str, repo: str, token: Optional[str] = None):
        self.run_id = run_id
        self.repo = repo
        self.token = token or self._get_gh_token()
        self.headers = {
            "Authorization": f"Bearer {self.token}",
            "Accept": "application/vnd.github+json",
            "X-GitHub-Api-Version": "2022-11-28"
        }

    def _get_gh_token(self) -> str:
        """Get GitHub token from gh CLI."""
        try:
            result = subprocess.run(
                ["gh", "auth", "token"],
                capture_output=True,
                text=True,
                check=True
            )
            return result.stdout.strip()
        except Exception as e:
            print(f"ERROR: Could not get GitHub token: {e}")
            print("Run: gh auth login")
            sys.exit(1)

    def wait_for_tmate_session(self, timeout: int = 600) -> Optional[Tuple[str, int]]:
        """
        Monitor CI logs for tmate session details.
        
        Returns:
            Tuple of (hostname, port) or None if not found
        """
        print(f"Monitoring run {self.run_id} for tmate session...")
        print(f"Timeout: {timeout}s")
        print()
        
        start_time = time.time()
        
        while time.time() - start_time < timeout:
            # Get workflow run status
            url = f"https://api.github.com/repos/{self.repo}/actions/runs/{self.run_id}"
            response = requests.get(url, headers=self.headers)
            
            if response.status_code != 200:
                print(f"ERROR: Failed to get run status: {response.status_code}")
                time.sleep(10)
                continue
            
            run_data = response.json()
            status = run_data.get("status")
            
            if status == "completed":
                print("Run completed before tmate session found")
                return None
            
            # Get job logs
            jobs_url = f"https://api.github.com/repos/{self.repo}/actions/runs/{self.run_id}/jobs"
            jobs_response = requests.get(jobs_url, headers=self.headers)
            
            if jobs_response.status_code != 200:
                time.sleep(10)
                continue
            
            jobs = jobs_response.json().get("jobs", [])
            
            for job in jobs:
                # Get logs URL
                logs_url = f"https://api.github.com/repos/{self.repo}/actions/jobs/{job['id']}/logs"
                logs_response = requests.get(logs_url, headers=self.headers, allow_redirects=True)
                
                if logs_response.status_code == 200:
                    logs = logs_response.text
                    
                    # Look for tmate SSH command
                    # Format: ssh <session>@<host>
                    ssh_match = re.search(r'ssh\s+([^@\s]+)@([^\s]+)', logs)
                    if ssh_match:
                        session = ssh_match.group(1)
                        host = ssh_match.group(2)
                        
                        # tmate uses default SSH port 22
                        print(f"✅ Found tmate session!")
                        print(f"   Host: {host}")
                        print(f"   Session: {session}")
                        return (f"{session}@{host}", 22)
            
            elapsed = int(time.time() - start_time)
            print(f"  [{elapsed}s] Waiting for tmate session...", end='\r')
            time.sleep(15)
        
        print("\nTimeout waiting for tmate session")
        return None

    def run_debug_commands(self, ssh_host: str, ssh_port: int):
        """Connect via SSH and run debug commands."""
        print()
        print("=" * 80)
        print("CONNECTING TO TMATE SESSION")
        print("=" * 80)
        print()
        
        client = paramiko.SSHClient()
        client.set_missing_host_key_policy(paramiko.AutoAddPolicy())
        
        try:
            # Connect (tmate uses key-based auth, no password needed)
            print(f"Connecting to {ssh_host}:{ssh_port}...")
            client.connect(
                hostname=ssh_host.split('@')[1] if '@' in ssh_host else ssh_host,
                port=ssh_port,
                username=ssh_host.split('@')[0] if '@' in ssh_host else 'runner',
                look_for_keys=True,
                timeout=30
            )
            print("✅ Connected!")
            print()
            
            # Commands to run
            commands = [
                ("Get webapi image", """
kubectl get deployment/nebari-landing-webapi -n nebari-system \
  -o jsonpath='{.spec.template.spec.containers[0].image}'
"""),
                ("Get webapi pod", """
kubectl get pod -n nebari-system \
  -l app.kubernetes.io/name=nebari-landing-webapi \
  -o jsonpath='{.items[0].metadata.name}'
"""),
                ("Extract and check binary for debug strings", """
POD=$(kubectl get pod -n nebari-system \
  -l app.kubernetes.io/name=nebari-landing-webapi \
  -o jsonpath='{.items[0].metadata.name}')
kubectl cp nebari-system/$POD:/app/webapi /tmp/webapi-binary 2>/dev/null
echo "=== Checking for [CACHE-DEBUG] ==="
if strings /tmp/webapi-binary | grep -q "CACHE-DEBUG"; then
  echo "✅ Found debug strings"
  strings /tmp/webapi-binary | grep "CACHE-DEBUG" | head -3
else
  echo "❌ NO debug strings found"
fi
echo ""
echo "=== Checking for [API] debug ==="
if strings /tmp/webapi-binary | grep -q "handleGetServices called"; then
  echo "✅ Found API debug strings"
else
  echo "❌ NO API debug strings found"
fi
"""),
                ("Check webapi logs for debug output", """
echo "=== Recent logs ==="
kubectl logs -n nebari-system \
  -l app.kubernetes.io/name=nebari-landing-webapi \
  --tail=30 | grep -E "INFO|ERROR|CACHE-DEBUG|API" | tail -20

echo ""
echo "=== Checking for [CACHE-DEBUG] in logs ==="
if kubectl logs -n nebari-system \
  -l app.kubernetes.io/name=nebari-landing-webapi \
  | grep -q "CACHE-DEBUG"; then
  echo "✅ Found in logs"
  kubectl logs -n nebari-system \
    -l app.kubernetes.io/name=nebari-landing-webapi \
    | grep "CACHE-DEBUG" | head -5
else
  echo "❌ NOT found in logs"
fi
"""),
            ]
            
            for description, command in commands:
                print("=" * 80)
                print(f"RUNNING: {description}")
                print("=" * 80)
                
                stdin, stdout, stderr = client.exec_command(command)
                exit_status = stdout.channel.recv_exit_status()
                
                output = stdout.read().decode('utf-8')
                error = stderr.read().decode('utf-8')
                
                if output:
                    print(output)
                if error:
                    print("STDERR:", error)
                
                print()
            
            print("=" * 80)
            print("VERIFICATION COMPLETE")
            print("=" * 80)
            
        except Exception as e:
            print(f"ERROR: {e}")
            import traceback
            traceback.print_exc()
        finally:
            client.close()


def main():
    parser = argparse.ArgumentParser(
        description="Automated tmate SSH debugging for CI"
    )
    parser.add_argument(
        "--run-id",
        required=True,
        help="GitHub Actions run ID"
    )
    parser.add_argument(
        "--repo",
        default="nebari-dev/nebari-landing",
        help="Repository (owner/name)"
    )
    parser.add_argument(
        "--token",
        help="GitHub token (default: from gh CLI)"
    )
    parser.add_argument(
        "--timeout",
        type=int,
        default=600,
        help="Timeout in seconds to wait for tmate session"
    )
    
    args = parser.parse_args()
    
    debugger = TmateDebugger(args.run_id, args.repo, args.token)
    
    # Wait for tmate session
    session_info = debugger.wait_for_tmate_session(timeout=args.timeout)
    
    if session_info:
        host, port = session_info
        debugger.run_debug_commands(host, port)
    else:
        print("Failed to find tmate session")
        sys.exit(1)


if __name__ == "__main__":
    main()
