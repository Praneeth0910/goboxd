import json
import os
import re

log_path = '/home/praneeth_0910/.gemini/antigravity-ide/brain/2d573289-39fd-454c-813b-f30b6aa3952f/.system_generated/tasks/task-75.log'
out_md = '/home/praneeth_0910/goboxd/stage2/payload_jsons.md'

with open(log_path, 'r') as f:
    log_content = f.read()

# Extract blocks of JSON from log
# Pattern: "  lang / filename.json    ✓ PASS... \n{" to next "\n\n"
matches = re.finditer(r'  ([a-z]+) / ([a-z_]+\.json)\s+.*?PASS.*?\n(\{.*?\n\})', log_content, re.DOTALL)

outputs = {}
for m in matches:
    lang = m.group(1)
    fname = m.group(2)
    out_json = m.group(3)
    try:
        # verify it's valid json
        json.loads(out_json)
        outputs[f"{lang}/{fname}"] = out_json
    except:
        pass

langs = ['scala', 'swift', 'typescript']
tests = ['accepted.json', 'build_failed.json', 'runtime_error.json', 'time_exceeded.json', 'wrong_output.json']

md = "# JSON Payloads\n\nThis document contains the JSON payload requests and their corresponding output responses for the Scala, Swift, and TypeScript test cases.\n\n"

for lang in langs:
    md += f"## {lang.capitalize()}\n\n"
    for test in tests:
        key = f"{lang}/{test}"
        md += f"### {key}\n\n"
        
        # Input
        in_path = os.path.join('/home/praneeth_0910/goboxd/payloads', lang, test)
        if os.path.exists(in_path):
            with open(in_path, 'r') as f:
                in_json = f.read().strip()
            md += "**Request Payload:**\n"
            md += "```json\n" + in_json + "\n```\n\n"
            
        # Output
        if key in outputs:
            md += "**Response Output:**\n"
            md += "```json\n" + outputs[key].strip() + "\n```\n\n"

with open(out_md, 'w') as f:
    f.write(md)

print("Generated MD successfully")
