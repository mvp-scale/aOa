Here's the full guide on how aOa extracts thinking from Claude responses.                                                                                                                                                                                                                                                                                                                                                                                             
                                       
  ---                                                                                                                                                                                                                                                                                                                                                                                                                                                                   
  Extracting Thinking from Claude Code Sessions
                                                                                                                                                                                                                                                                                                                                                                                                                                                                        
  Source Data                          
                                                                                                
  Claude Code stores sessions as JSONL files at:
  ~/.claude/projects/{projectPath}/{sessionId}.jsonl

  Each line is one JSON object. An assistant response looks like this:

  {
    "type": "assistant",
    "timestamp": "2026-02-14T15:30:45.123Z",
    "message": {
      "role": "assistant",
      "content": [
        {
          "type": "thinking",
          "thinking": "Let me analyze this function..."
        },
        {
          "type": "text",
          "text": "Here's what I found..."
        },
        {
          "type": "tool_use",
          "id": "toolu_01ABC...",
          "name": "Read",
          "input": {"file_path": "/src/foo.py"}
        }
      ]
    }
  }

  The key insight: message.content is an array of content blocks. Thinking is just another block type alongside text and tool_use.

  The Three Content Block Types

  ┌────────────┬───────────────────────┬─────────────────────────────────────────────────────────────────┐
  │    type    │ Field containing text │                           What it is                            │
  ├────────────┼───────────────────────┼─────────────────────────────────────────────────────────────────┤
  │ "thinking" │ .thinking             │ Claude's reasoning (hidden from user, but generated and stored) │
  ├────────────┼───────────────────────┼─────────────────────────────────────────────────────────────────┤
  │ "text"     │ .text                 │ Visible response text                                           │
  ├────────────┼───────────────────────┼─────────────────────────────────────────────────────────────────┤
  │ "tool_use" │ .input (JSON object)  │ Tool call payload                                               │
  └────────────┴───────────────────────┴─────────────────────────────────────────────────────────────────┘

  Critical detail: For thinking blocks, the text is in the .thinking field, NOT .text. For text blocks, it's in .text. This is an asymmetry in the Claude response format.

  Extraction Code (Python — indexer.py:5381-5395)

  # Assistant thinking + text
  elif msg_type == "assistant":
      content = msg.get("content", [])
      if isinstance(content, list):
          for item in content:
              if isinstance(item, dict):
                  item_type = item.get("type")
                  if item_type == "thinking":
                      thinking = item.get("thinking", "")  # <-- .thinking, NOT .text
                      if thinking and len(thinking) > 10:
                          texts.append({"type": "thinking", "text": thinking, "ts": timestamp})
                  elif item_type == "text":
                      text = item.get("text", "")           # <-- .text
                      if text and len(text) > 10:
                          texts.append({"type": "output", "text": text, "ts": timestamp})

  Filtering Rules

  1. Only process lines where data.type == "assistant" — skip user messages, tool results, meta entries
  2. message.content must be a list — sometimes it's a string (user messages), guard against that
  3. Minimum length filter: len(thinking) > 10 — skip trivially short thinking blocks (e.g., empty "" or "\n")
  4. Skip isMeta entries: User messages check not data.get("isMeta") to filter system-generated noise
  5. Timestamp filter: The since parameter avoids re-processing already-seen content

  Token Counting (for metrics — metrics.py:88-132)

  for item in content:
      item_type = item.get("type", "")
      if item_type == "thinking":
          thinking_text = item.get("thinking", "")   # .thinking field
          if thinking_text:
              total_chars += len(thinking_text)
      elif item_type == "text":
          text = item.get("text", "")                 # .text field
          if text:
              total_chars += len(text)
      elif item_type == "tool_use":
          tool_input = item.get("input", {})
          if tool_input:
              total_chars += len(json.dumps(tool_input))

  return total_chars // 4  # ~4 chars per token

  Display in aoa cc conversation (30-analytics.sh:711-783)

  The CLI groups content into turns (prompt starts each turn), then renders:

  ─── 1 ─────────────────────────────
  You ▸ What does this function do?        ← YELLOW

  Claude ▸ Here's what I found...          ← GREEN
      └─ Let me analyze this function...   ← DIM (thinking, nested)

  ─── ▶ NOW ──────────────────────────
  You ▸ Can you fix the bug?               ← YELLOW

  The jq extraction at line 755:
  (.value | map(select(.type == "thinking")) | .[] | "THINK\t\(.text)")

  For the Go Port

  The critical mapping:

  type ContentBlock struct {
      Type     string          `json:"type"`      // "thinking", "text", "tool_use"
      Text     string          `json:"text"`      // populated for type="text"
      Thinking string          `json:"thinking"`  // populated for type="thinking"
      ID       string          `json:"id"`        // tool_use ID
      Name     string          `json:"name"`      // tool name
      Input    json.RawMessage `json:"input"`     // tool input
  }

  // Extract thinking:
  for _, block := range msg.Content {
      switch block.Type {
      case "thinking":
          text = block.Thinking  // NOT block.Text
      case "text":
          text = block.Text
      case "tool_use":
          text = string(block.Input)
      }
  }

  The field asymmetry (thinking.thinking vs text.text) is the one thing that trips up every parser. The research doc at aOa-go/research/session-log-format.md also documents known bugs — duplicate entries, corrupt parentUuid references, and bloated persisted-output entries that you need to guard against.

✻ Crunched for 55s                                                                                                                                                                                                                                                                                                             