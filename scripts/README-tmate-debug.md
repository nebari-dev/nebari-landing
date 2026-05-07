# Automated tmate Debugging for CI

Python script to automatically connect to tmate sessions in GitHub Actions and run verification commands.

## Setup

```bash
# Install dependencies
pip install paramiko requests

# Ensure gh CLI is authenticated
gh auth status
# If not authenticated:
# gh auth login
```

## Usage

### 1. Trigger CI workflow with tmate session
The e2e.yml workflow includes a tmate step that will pause before running tests.

### 2. Run the automated debugger

```bash
# Get the run ID from GitHub Actions
# Example: https://github.com/nebari-dev/nebari-landing/actions/runs/25512392464
#          The run ID is: 25512392464

python3 scripts/tmate-debug.py --run-id 25512392464
```

The script will:
1. Monitor the CI run for tmate session details
2. Extract SSH connection information
3. Connect automatically via paramiko
4. Run verification commands:
   - Check webapi image and pod info
   - Extract binary and search for debug strings
   - Check logs for debug output
5. Report findings

## What It Checks

### Debug Strings in Binary
- `[CACHE-DEBUG]` - Cache add/update operations
- `handleGetServices called` - API handler logging

### Debug Output in Logs
- `[CACHE-DEBUG] Service added/updated`
- `[API] handleGetServices called`
- Service visibility and access checks

## Expected Results

### ✅ If Quay Cache NOT the Issue
```
✅ Found debug strings
✅ Found in logs
[CACHE-DEBUG] Service added/updated: visibility=public
[API] handleGetServices called
```

### ❌ If Quay Serving Stale Image
```
❌ NO debug strings found
❌ NOT found in logs
```

## Troubleshooting

### SSH Connection Fails
```python
# tmate uses SSH keys from the GitHub actor
# Ensure your SSH agent has keys loaded:
ssh-add -l

# If empty, add your key:
ssh-add ~/.ssh/id_rsa  # or id_ed25519, etc.
```

### Timeout Before Session Found
```bash
# Increase timeout (default: 600s)
python3 scripts/tmate-debug.py --run-id <ID> --timeout 900
```

### GitHub API Rate Limit
```bash
# Use a personal access token instead of gh CLI
export GITHUB_TOKEN=ghp_yourtoken
python3 scripts/tmate-debug.py --run-id <ID> --token $GITHUB_TOKEN
```

## Manual Connection (Fallback)

If the script fails, you can still connect manually:

1. Watch CI logs for: `ssh <session>@nyc1.tmate.io`
2. Copy and run that command
3. Run verification commands from `~/.copilot/session-state/*/files/tmate-debug-guide.md`

## Alternative: Inline Verification

If you prefer automated checks without SSH, see git history for inline verification commit (9b401ac).
