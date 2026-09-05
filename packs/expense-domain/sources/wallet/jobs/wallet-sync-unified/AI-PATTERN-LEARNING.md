# AI-Assisted Pattern Learning for Email Extraction

## Overview

When the extract-engine encounters an email that doesn't match any existing format pattern, it can automatically call an AI provider (DeepSeek or Claude) to:
1. Analyze the email structure and content
2. Suggest a complete regex pattern with named capture groups
3. Generate and save a new format file automatically
4. Make the pattern immediately available for subsequent emails

## Quick Start

### Setup

1. **Configure AI provider** (DeepSeek or Claude):
   ```bash
   # Copy the example config
   cp ~/automation-monorepo-config/config/ai/deepseek.example.yaml ~/automation-monorepo-config/config/ai/deepseek.yaml
   
   # Edit and fill in your API key
   nano ~/automation-monorepo-config/config/ai/deepseek.yaml
   ```

2. **Set environment variables**:
   ```bash
   export CONFIG_PATH=~/automation-monorepo-config
   export AI_PROVIDER=deepseek
   export DEEPSEEK_API_KEY="sk-..."
   ```

### Usage

#### Direct CLI

```bash
# Run extraction with AI-assisted learning enabled
cd packs/expense-domain/sources/wallet/jobs/wallet-sync-unified

python3 extract-engine.py --file unmatched-emails.jsonl --ai-assist
```

#### Via Framework Runner

```bash
# Using the configuration from AI profile
CONFIG_PATH=~/automation-monorepo-config .auto run wallet-sync --ai deepseek -- --ai-assist
```

## How It Works

### 1. Unmatched Email Detection
When an email doesn't match any format:
```json
{
  "matched": false,
  "action": "unmatched",
  "sender": "alerts@newbank.com",
  "subject": "Payment Alert",
  "unmatched_excerpt": "..."
}
```

### 2. AI Analysis
The engine sends the email to the AI provider with a prompt asking for:
- Email classification (bank alert, payment confirmation, etc.)
- Complete regex pattern with named capture groups
- Field extraction guidance

### 3. Pattern Generation
AI returns a pattern like:
```
"Dear Customer,\\s*Your payment of\\s*[₹$]*\\s*(?P<amount>[\\d,]+(?:\\.\\d{2})?)"
```

Named groups: `(?P<name>...)` for fields to extract

### 4. Format File Creation
Engine creates a new format file at:
```
~/automation-monorepo-config/config/gmail/email-formats/email.{format-name}.yaml
```

Example output:
```yaml
name: newbank-payment-alert
source: gmail
priority: 90
match:
  sender: alerts@newbank\.com
  subject: Payment
action: extract
fields:
  body: >-
    Dear Customer,\s*Your payment of...
transforms:
  amount: decimal
  date:
    type: date
    formats: ["%d-%m-%y", "%d-%m-%Y", "%Y-%m-%d"]
set:
  currency: INR
  direction: debit
```

### 5. Immediate Availability
The new pattern is immediately available for:
- Subsequent emails in the same run
- Future runs (format files are persistent)

## Output

When `--ai-assist` is enabled, unmatched emails include:

```json
{
  "matched": false,
  "action": "unmatched",
  "ai_suggestion": {
    "success": true,
    "format_name": "newbank-payment-alert",
    "body_pattern": "...",
    "reasoning": "..."
  },
  "ai_created_format_file": "/path/to/email.newbank-payment-alert.yaml"
}
```

## Best Practices

### 1. Review Generated Patterns
Before deploying in production:
- Check the suggested regex pattern for accuracy
- Verify the extracted fields make sense
- Test with sample emails of the same type

### 2. Iterate and Refine
If a pattern needs adjustment:
1. Edit the generated YAML file
2. Test with sample emails
3. Lower the priority if conflicts with existing patterns

### 3. Cost Management
- AI API calls incur usage charges
- Use `--ai-assist` selectively, not on every sync
- Run in batch mode with multiple unmatched emails to amortize cost

### 4. Pattern Priority
Generated patterns use priority 90 (higher = earlier evaluation). Adjust if needed:
```yaml
priority: 50  # Lower priority to evaluate after manual patterns
priority: 90  # Higher priority to evaluate first
```

## Troubleshooting

### API Connection Errors
```
error: "Connection refused" / "Timeout"
```
- Verify API_KEY is correct
- Check network connectivity
- Verify API endpoint is accessible

### Pattern Matching Issues
If a generated pattern doesn't extract correctly:
1. Check the actual email body against the pattern
2. Verify named capture groups are present
3. Test regex independently: `python3 -c "import re; print(re.search(pattern, text).groupdict())"`

### Unicode/Encoding Issues
- Currency symbols (₹, €, $) are handled but may need adjustment
- If pattern contains unusual characters, edit the YAML manually
- Use escaped forms: `\\u20b9` for rupee symbol

## Supported AI Providers

### DeepSeek
```yaml
provider: deepseek
api_key: sk-...
model: deepseek-chat  # default
api_base: https://api.deepseek.com  # optional override
```

### Claude (Anthropic)
```yaml
provider: claude
api_key: sk-ant-...
model: claude-3-5-sonnet-20241022  # default
```

## Advanced: Custom Prompts

To modify the AI analysis, edit the `suggest_pattern_via_ai()` function in `extract-engine.py` and adjust the prompt text.

## Example: End-to-End Flow

```bash
# 1. Setup
export CONFIG_PATH=~/automation-monorepo-config
export AI_PROVIDER=deepseek
export DEEPSEEK_API_KEY="sk-..."

# 2. Create test email (unmatched)
echo '{"source":"gmail","id":"test_001","sender":"alerts@newbank.com","subject":"Payment Confirmed","date":"2026-09-05T12:00:00Z","body":"Your payment of ₹5000.00 was processed on 05-SEP-26"}' > /tmp/test.jsonl

# 3. Run with AI assist
python3 extract-engine.py --file /tmp/test.jsonl --ai-assist

# 4. Check the output
# - ai_suggestion field shows AI analysis
# - ai_created_format_file shows path to new format

# 5. Re-run without --ai-assist (uses new format)
python3 extract-engine.py --file /tmp/test.jsonl
# Now matched and extracted!
```

## Security

- **API Keys**: Store in `~/automation-monorepo-config/config/ai/` (git-ignored)
- **Email Content**: Sent to AI provider; review privacy policy if sensitive data involved
- **Generated Files**: Review generated YAML before deploying to production
