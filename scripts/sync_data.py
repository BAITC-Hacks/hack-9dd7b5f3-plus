#!/usr/bin/env python3
"""Copy only runtime files into the backend Docker build context. Do not modify data/."""
from pathlib import Path
import hashlib
import json
import shutil
root = Path(__file__).resolve().parents[1]
target = root / 'backend/data/official'
target.mkdir(parents=True, exist_ok=True)
checksums = {}
for name in ['scenarios.json','slots.json','actions.json','knowledge_base.json','mock_backend.json']:
    source = root / 'data' / name
    shutil.copyfile(source, target / name)
    checksums[name] = hashlib.sha256(source.read_bytes()).hexdigest()
(target / 'checksums.json').write_text(json.dumps(checksums, indent=2)+'\n')
print('Synced runtime catalog/facts only; no evaluation utterances copied.')
