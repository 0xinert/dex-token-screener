import json

INPUT_FILE = "input.json"
OUTPUT_FILE = "output.json"
DECIMALS = 18

with open("parsed_10000_BSC_tokens.json") as f:
    data = json.load(f)

result = []

for symbol, tokens in data.items():
    for t in tokens:
        result.append({
            "contract_address": t["contract_address"],
            "decimals": DECIMALS,
            "name": symbol,     # or t["name"] if you prefer
            "symbol": symbol
        })

with open(OUTPUT_FILE, "w") as f:
    json.dump(result, f, indent=2)

print(f"Generated {len(result)} tokens")
