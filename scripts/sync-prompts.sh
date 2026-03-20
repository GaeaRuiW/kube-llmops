#!/usr/bin/env bash
set -euo pipefail

# Sync RAG prompts from Git to Langfuse
# Usage:
#   ./scripts/sync-prompts.sh                    # Sync all prompts
#   LANGFUSE_HOST=http://localhost:3001 ./scripts/sync-prompts.sh  # Custom host

LANGFUSE_HOST="${LANGFUSE_HOST:-http://kube-llmops-langfuse:3000}"
LANGFUSE_PK="${LANGFUSE_PUBLIC_KEY:-pk-lf-kube-llmops}"
LANGFUSE_SK="${LANGFUSE_SECRET_KEY:-sk-lf-kube-llmops}"
PROMPTS_FILE="${1:-$(dirname "$0")/../examples/prompts/rag-system-prompts.json}"

echo "============================================="
echo "  Prompt Sync: Git → Langfuse"
echo "============================================="
echo "  Host:    ${LANGFUSE_HOST}"
echo "  File:    ${PROMPTS_FILE}"
echo ""

if [ ! -f "$PROMPTS_FILE" ]; then
  echo "ERROR: Prompts file not found: $PROMPTS_FILE"
  exit 1
fi

AUTH=$(echo -n "${LANGFUSE_PK}:${LANGFUSE_SK}" | base64)

# Read prompts and sync each one
python3 -c "
import json, sys, os
from urllib.request import Request, urlopen
from urllib.error import URLError, HTTPError
import base64

host = os.environ['LANGFUSE_HOST']
pk = os.environ['LANGFUSE_PK']
sk = os.environ['LANGFUSE_SK']
auth = base64.b64encode(f'{pk}:{sk}'.encode()).decode()

with open('$PROMPTS_FILE') as f:
    data = json.load(f)

for prompt in data['prompts']:
    name = prompt['name']
    # Create or update prompt in Langfuse
    body = json.dumps({
        'name': name,
        'prompt': prompt['template'],
        'config': {'description': prompt.get('description', '')},
        'labels': ['rag', f'v{prompt.get(\"version\", 1)}'],
    }).encode()
    
    req = Request(
        f'{host}/api/public/v2/prompts',
        data=body,
        headers={
            'Content-Type': 'application/json',
            'Authorization': f'Basic {auth}',
        },
        method='POST'
    )
    
    try:
        resp = urlopen(req, timeout=10)
        print(f'  SYNCED: {name} (v{prompt.get(\"version\", 1)})')
    except HTTPError as e:
        if e.code == 409:
            print(f'  EXISTS: {name} (already up to date)')
        else:
            print(f'  FAILED: {name} ({e.code}: {e.read().decode()[:100]})')
    except Exception as e:
        print(f'  ERROR: {name} ({e})')

print()
print('Prompt sync complete.')
" 

echo ""
echo "View prompts in Langfuse: ${LANGFUSE_HOST}/prompts"
