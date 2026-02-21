# aOa System Feedback Outline

> Put your notes under any section. Leave sections blank if nothing to say.
> When done, hand this back and it gets ingested into the board.

---

## 1. Web Dashboard

### 1.1 Global (Header / Footer / Navigation)

**What's there now:** Header with aOa brand logo, three tab buttons (Overview | Learning | Conversation), connection status badge (connected/disconnected/mock), dark/light theme toggle. Footer with online indicator, version, motto, live clock.

_Notes:_
I think this is great. The color scheme, AOA branding is blue. We have overview, lean, conversation. I think we could put some tentative forward-thinking tabs here. I think a composable dashboard is probably the one that I think users would love. So the overview custom dashboard, and the idea is that we probably have to figure out through the angles of attack the naming convention here. We don't, we're not aligned. And so each one of these tabs could be that style of angle attack to keep the branding on point.


### 1.2 Overview Tab

#### 1.2.1 Hero Card
Top-left. Brand narrative with animated domain count, prompt count, file count woven into prose. "ANGLE OF ATTACK" headline.

_Notes:_
I love this one. It really tells a story, but I think the Agentic work optimized to 01, this is great. And I think we're failing to deliver on the promise of speed and token savings. It's not mentioned here. This is typically why you would come here. And so we have to represent the speed token savings first and foremost in this section while saying the agentic work optimized to 01 with a little flare around the angle of attack and its points. You have some sentence with numerical values. I think that could work if delivered. And perhaps that's where it's the goal-oriented use of the metrics that talk about speed, that talk about you're on your way to getting something. And on the right over here, where we have the particular card, the metric panel for prompt and active, I like it, but in this, the number of prompts and actives not necessarily driving any value to the customer. Yes, data, but it doesn't show token savings, speed savings, time savings, anything that can be in that area. Auto-tunes cool, but we might be able to put that in a smaller card down below.


#### 1.2.2 Metrics Panel
Top-right. Two large numbers (prompts, domains). Autotune progress bar (X/50, next cycle countdown). Uptime chip.

_Notes:_
I love this one. It really tells a story, but I think the Agentic work optimized to 01, this is great. And I think we're failing to deliver on the promise of speed and token savings. It's not mentioned here. This is typically why you would come here. And so we have to represent the speed token savings first and foremost in this section while saying the agentic work optimized to 01 with a little flare around the angle of attack and its points. You have some sentence with numerical values. I think that could work if delivered. And perhaps that's where it's the goal-oriented use of the metrics that talk about speed, that talk about you're on your way to getting something. And on the right over here, where we have the particular card, the metric panel for prompt and active, I like it, but in this, the number of prompts and actives not necessarily driving any value to the customer. Yes, data, but it doesn't show token savings, speed savings, time savings, anything that can be in that area. Auto-tunes cool, but we might be able to put that in a smaller card down below.

#### 1.2.3 Stats Grid (6 cards)
Row of six color-coded stat cards: Keywords (cyan), Terms (green), Bigrams (purple), File Hits (yellow), Files Indexed (blue), Tokens (cyan).

_Notes:_
I like the cards, but we really need to be able to hit home on the value of what we're doing here. Yeah, we have data, but showing data for data's sake doesn't really deliver home. What exactly are we telling them about the overview in this page? We have the activities, the impact. We have some information that we have already done. They have index files, average speed of queries. We use that value, the rolling average or the averages of what we're saving them per response, the time savings, the token savings. We have ways of representing it here that we don't in these cards, as well as their entire portfolio is under view. And so, as we look towards the future, AOA would be able to help identify some of the different value adds. And so, some of these particular cards could align to the security and the conformity and performance areas as well, all right, found in your code right now. And so, we could bring that together here. That's all here, all right, part of that sort of value prop. You'll have to look at that. It's not coded, but that would be powerful to bring that conversation in.

#### 1.2.4 Activity & Impact Table
Bottom card. Table with columns: Time, Source, Action, Attrib, aOa Impact, Target. Populated from activity ring buffer. Color-coded attribs (cyan=indexed, yellow=regex, green=multi-*). Badge shows entry count.

_Notes:_
I love this card. It needs to be stretched down so you have more elements. The navigation scroll bar off to the side should be a little bit more identifiable, but overall, this is very clean. We can get rid of tags and we've already talked about it. We have to address in this activity's impact of how we're doing it, we have to address all possible states of what actions come in, the source of doing it, the attribution, the AOA impact. All right, whether or not we put time all the way to the left to allow for targets, which will sometimes be long file names, to be represented in this field here to show when there's value opportunities to highlight this target, to highlight it. We've talked about this in the past. This is a very clean section. It's just not long enough. It doesn't show enough information. And it should auto-scroll. Maybe hinting at where now is. Oh, you see the time. I think the now time, if it goes off to the left, perhaps we can indicate cleanly where now is.

### 1.3 Learning Tab

#### 1.3.1 Stats Grid (6 cards)
Domains (purple), Core (green), Terms (cyan), Keywords (blue), Bigrams (yellow), Total Hits (red).

_Notes:_
Again, pretty good tab. I think this one's pretty good. We have some blank space down at the bottom. Not to say that we have to use it. I think we optimize it quite good here. I think the overall domain hits and terms, the delight is when terms are found during searching and everything, they light up, as well as the ngrams and the project was it, the co-hits. And so it's all pretty good. But how are we showing the value of learning here? It's counting. And so the metric cards up top, the stat grids, these, what do we have that represents actual learning? It's the competition on the top domains. We don't show the competitive sorting in the domains. We have core, but it's the cards here, they're not focused on that desired goal. The goal here is to educate the user on how we are learning. What stat cards can we show currently that represent that in a very good way? There's some that are good here, and we're going to have some that are just numeric, but we need to focus and make sure it answers the question.

#### 1.3.2 Domain Rankings Card (left column)
Table sorted by learning signal strength. Each domain row shows term pills (up to 10 per row, sorted by keyword hit popularity). Pills are gray by default, flash green on hit with 30-second fade. Footer: "aOa learns your workflow..."

_Notes:_
I like this one. I think we can say sorted by learn signal strength. I think we can say it in a better way and let them know it's sorted by their intent. I think we could highlight that somehow, even if it's symbolic. And so instead of just saying the text, I think we might be able to say intent score, and then that's the sorting mechanism. Instead of just the numerical domain, it's some kind of hint towards intent learning, you know, self-learning algorithm. We can put some scoring there. We don't need to just show numbers. We can make a value prop out of the numerical values here of representing it in a better way.

#### 1.3.3 N-gram Metrics Card (right column)
Three sections: Bigrams, Cohit KW-to-Term, Cohit Term-to-Domain. Each row has name, visual bar, and count. Rows flash green on value change (3s). Count gets blue glow on change (4s).

_Notes:_
I think this section's great. I think it delivers everything that it needs. Yeah.

### 1.4 Conversation Tab

#### 1.4.1 Hero Narrative
Top section. "Session Activity" headline. Animated stats: turn count, token count, cache efficiency percentage.

_Notes:_
This section I struggle with. This is about conversation. I like the hero card, the hero narrative, but we have to bring it back home to what AOA is trying to do. AOA is trying to not only save you time, save you tokens, but in this case, it's informing you. And this is the hidden conversation that's going on behind every prompt. Much the way we did in the overview, we got to bring that card, that hero card, to light in the same way and bring that value prop of the conversation or whatever we call this that's aligning towards this angle of attent, that's bringing that hidden value to you so you understand everything that's going on.

#### 1.4.2 Stats Grid (5 cards)
Input Tokens (cyan), Output Tokens (blue), Cache Read (green), Cache Write (purple), Cache Hit % (yellow).

_Notes:_
Now these stack grids, these are great and I think overall we have some good stack grids, but we need to make sure that it aligns to the conversation. We have the data, but are we applying it and using it correctly to represent this conversation structure? Input tokens, output tokens, cash reads. All great. Could be totals, you know, could be rates, could be velocity, could be value. Why, you know, we see 100% cash hit. Why aren't we representing AOA here in our token savings? Why aren't we bringing in and saving them? They're saving time. We don't have it.

#### 1.4.3 Conversation Feed Card
Two-column card with yellow border. Bottom-anchored scroll.

**Left column (Messages):** User lines (yellow border), thinking lines (purple, click to expand, truncated to 500 chars), assistant responses (green, with model tag + timing + token count).

**Right column (Actions):** Tool chips color-coded by type, with target paths, range info, impact badges. Footer with tool/edit/guided counts.

**NOW bar** at bottom (green live indicator). "Jump to now" button when scrolled up. In-progress turn shown live via currentBuilder.

_Notes:_
The conversation feed, I think we can get rid of the yellow outline. It was good while it lasted. I'm thinking I can help you here, and this is a conversation. Don't just run with this one. Have a conversation. I think the problem that you're having in not showing live, real-time, prompt, assistant thinking in real time is you are waiting to collect all the tools. And if that is the problem, and only if that's the problem, then why wouldn't we have a two-thirds kind of section that shows the live conversation each turn, and then off to the right, just the tool calls, tool calls, and everything that's happening inside it separately. It's that way it's painted, everything's coming in fast and furious, and it shows the dynamic nature and the velocity of everything that's happening, and you see it.

Now you try to get the timing as much as you can. I think it's great. We may in fact be able to put that token the tokens and the savings in small white font above each section. I think there's ample space. So I don't know how to represent the You know what? It probably is better to move the time and tokens off to the left. Perhaps that could be a slight information section off to the left. Let's have a conversation on what is possible. If it's not consistent, then I don't think we need it.

---

## 2. CLI Commands

### 2.1 Search Commands

#### 2.1.1 `aoa grep [flags] <query>`
O(1) indexed symbol search + content body scan. Modes: literal, OR (space-separated), AND (-a, comma-separated), case-insensitive (-i), word-boundary (-w). Output: symbol hits then content hits, with domains, tags, file:line. Max 20 results default.

_Notes:_
Understand, here's the desired goal. When we're done, we're going to be able to create an alias for AOA grep, all right, as grep. And they're going to use AOA grep everywhere in the system. And that is the requirement to be able to service grep as though it were grep. This allows us to trick any AI agent or tool assisted agent to using our tool and providing very little instructions. We offer some instructions when they run a command that may not work to guide them to maybe a help menu or a value menu, something. We don't want to have to give a lot of usage guys or files and bloat up the prompt. We want to be able to trick the use of grep as a first-class citizen as well as e-grep.

#### 2.1.2 `aoa egrep [flags] <pattern>`
Regex search across all symbols. Shares flags: -c, -q, -m, -e, --include, --exclude.

_Notes:_
Understand, here's the desired goal. When we're done, we're going to be able to create an alias for AOA grep, all right, as grep. And they're going to use AOA grep everywhere in the system. And that is the requirement to be able to service grep as though it were grep. This allows us to trick any AI agent or tool assisted agent to using our tool and providing very little instructions. We offer some instructions when they run a command that may not work to guide them to maybe a help menu or a value menu, something. We don't want to have to give a lot of usage guys or files and bloat up the prompt. We want to be able to trick the use of grep as a first-class citizen as well as e-grep.

#### 2.1.3 `aoa find <glob>`
Glob-based filename search across indexed files.

_Notes:_
When it comes to the commands that you're going to find below, AOA has the ability to find things faster. It also is respective of.getignore. Find and others do not respect the.getignore. We do. We would yet again attempt to mask Find in all of its complexities and alias it, and they would use our service, alright and we would mask Find as well. Again, giving them guidance on how to use the tool more effectively for code assistance.

#### 2.1.4 `aoa locate <name>`
Substring filename search across indexed files.

_Notes:_
Pretty much the same thing. I don't know if we need to mask everything because I don't think there is a locate. There's a witch that sits in most systems, but what could locate do? I think locate, in my mind, is kind of like a peak. If you know the file name and you know the section that you want to look at, that could be the locate. where you don't just give it a file name, you give it a file and method and then it just shows you the text in that section. That might be much more powerful and that might be AOA peak or some type of peak structure that we could use to educate the agents.

#### 2.1.5 `aoa tree [dir]`
Filesystem tree view. --depth flag. No daemon required. Ignores .git, node_modules, etc.

_Notes:_
A basic tree that respects git.

### 2.2 Daemon Management

#### 2.2.1 `aoa daemon start`
Spawns background daemon. Writes PID to .aoa/daemon.pid, logs to .aoa/daemon.log. Waits for socket reachability (10s timeout). Prints dashboard URL on success.

_Notes:_
the daemon that has all of the start, restart and commands. I think this is great. I think if you do AOA init, it should automatically start. About the only concern I have with the daemon is to make sure it can handle multiple projects at the same time.

#### 2.2.2 `aoa daemon stop`
Graceful shutdown via socket. Falls back to PID-based SIGTERM/SIGKILL. Cleans up stale files.

_Notes:_


#### 2.2.3 `aoa daemon restart`
Stop + 200ms pause + start.

_Notes:_


### 2.3 Project Management

#### 2.3.1 `aoa init`
Indexes current project. Tree-sitter parse of all code files, builds inverted index in bbolt. Skips .git, node_modules, vendor, etc. Skips files >1MB. Fails if daemon holds bbolt lock.

_Notes:_
The AOA init is our entry level into establishing the structures and the rules that we need. I think it's during AOA init we have to have a very guided structure that very simply asks, you know, do you want to replace git or grep with AOA? It allows for less instructions, saves tokens, and gives you all the benefits of AOA. And then we can ask them if there's a few others that they would want, hopefully only two or three, and then say, now you're aligned, no instructions required, save the prompts.  And at the end, if they want more information, it's AOA help.

#### 2.3.2 `aoa wipe [--force]`
Deletes all persisted index and learner state. Prompts for confirmation unless --force. Works with or without daemon (socket wipe or direct DB delete).

_Notes:_
This is always a good catch all for just cleaning up information and starting again.

### 2.4 Information Commands

#### 2.4.1 `aoa health`
Daemon status check. Shows status, file count, token count, uptime.

_Notes:_


#### 2.4.2 `aoa config`
Shows project root, DB path, socket path, daemon status, dashboard URL. No daemon required.

_Notes:_


#### 2.4.3 `aoa open`
Opens dashboard in default browser. Reads port from .aoa/http.port. Falls back to printing URL.

_Notes:_
Okay, I like the AOA open. Yeah, if it sets up a dashboard, somewhere in here we have to be able to create a port in case there's port confusion. So I don't know if it's AOA open or AOA web, but somewhere in here we have to create a very clean connection.

### 2.5 Output Formatting
Search results: two-section display (symbol hits, then content hits). Hit breakdown header shows (N symbol, M content). Symbol hits show domain, tags, signature. Content hits show enclosing symbol context + grep-style line content.

_Notes:_


---

## 3. Backend Systems

### 3.1 Domain Layer (dependency-free)

#### 3.1.1 Search Engine (`internal/domain/index/`)
O(1) token lookup via inverted index. Four modes: literal, OR, AND, regex. Content scanning (grep-style body search with enclosing symbol resolution). Domain enrichment from atlas. Tag generation (keywords resolved to atlas terms). Observer hook feeds search signals to learner.

_Notes:_
I'm pretty certain we cover this. We have everything Grep and egrep needs for searching and we have to be able to do everything they do hopefully at 90%. Some things may not be in our scope, and I'm willing to accept some, but it has to be the majority.

#### 3.1.2 Learner (`internal/domain/learner/`)
observe() processes signals: keywords -> terms -> domains -> cohits -> file hits. 21-step autotune every 50 prompts (lifecycle, curation, decay, pruning). Competitive displacement (top 24 core). Bigram extraction from conversation text. DomainMeta.Hits is float64 (critical precision rule). DecayRate=0.90, PruneFloor=0.3.

_Notes:_
Concern about the internal domain learner. I put this as a goal previously. We don't want to blow up. If you have a large code base or if you use it heavily, we need to be aware of memory constraints and performance. It should always perform as fast and as quick as possible. It should beat out much of the competition and express that where we can.

#### 3.1.3 Enricher (`internal/domain/enricher/`)
Atlas: 134 domains, 15 JSON files, 938 terms, 6566 keywords. O(1) keyword-to-term-to-domain lookup (~20ns). Shared keywords across domains. LookupTerm() reverse lookup from term name to owning domain(s).

_Notes:_
The enricher is the value prop. It changes the standard grep output and so our enricher is our value prop. This has to be guarded, performant, and well articulated in the code. So when we add enhancements or features or ports and adapters people know exactly what's happening here.

#### 3.1.4 Status Writer (`internal/domain/status/`)
Generates .aoa/status.json. Contains intents, domain count, top 3 domains, autotune deltas. Read by hook script for Claude Code status line.

_Notes:_
Is this helping the is this just the state? Are we not saving it in the database? I'm confused about the status, aoa status.json. I know we need something, because when the daemon's off, we need something to resurrect everything. And so perhaps that's what it is.

### 3.2 Adapters

#### 3.2.1 bbolt Store (`internal/adapters/bbolt/`)
Transactional B+ tree persistence. Project-scoped buckets with "index" and "learner" sub-buckets. Crash-safe writes. TokenRef encoded as "fileID:line" strings. Single-writer lock (source of init-vs-daemon contention).

_Notes:_
Um we I believe we already tried to battle test this and I continue to say it has to be battle tested um to make sure that it doesn't lock up or freeze or uh get hurt under large code bases or aggressive agents.

#### 3.2.2 Unix Socket Server (`internal/adapters/socket/`)
JSON-over-socket IPC. Stale socket cleanup. Concurrent client handling. AppQueries interface for learner snapshots, metrics, feeds. Client used by CLI commands.

_Notes:_
Um we I believe we already tried to battle test this and I continue to say it has to be battle tested um to make sure that it doesn't lock up or freeze or uh get hurt under large code bases or aggressive agents.

#### 3.2.3 HTTP/Web Server (`internal/adapters/web/`)
Localhost-only HTTP server. Embedded static files (index.html). 11 JSON API endpoints. Port = 19000 + (hash(project_root) % 1000). Writes port to .aoa/http.port.

_Notes:_
I think we're going to do well. If we codified this well, this should be able to handle many uh requests per second.

#### 3.2.4 Session Tailer (`internal/adapters/tailer/`)
Polls Claude Code JSONL session files at 500ms intervals. Auto-discovers ~/.claude/projects/{encoded-path}/. Finds most recent JSONL, seeks to end (skips history). UUID dedup prevents double-processing. Defensive parser rejects oversized lines, detects gaps.

_Notes:_
Yep, this area is also a guarded secret. This is where we get much of our data from, our n-grams, our everything. And so we use our defensive parsing technique. We try to pull out as much value as we can from this information. So we're always attempting to get information as good as we can and as accurate and timely as we can.

#### 3.2.5 Session Prism / Claude Reader (`internal/adapters/claude/`)
Translates raw Claude-specific JSONL into canonical ports.SessionEvent. Decomposes compound messages (thinking + text + tool_uses) into atomic events. One TurnID per AI turn. Tracks ExtractionHealth (yield, gaps, version changes).

_Notes:_
It's the architecture that sets us up with the adapters. So I see this just being a way to read it and it's it's probably exactly what it needs to be.

#### 3.2.6 Tree-sitter Parser (`internal/adapters/treesitter/`)
28 compiled-in language grammars via CGo. Extracts symbols (functions, classes, methods, structs, interfaces). Returns Symbol -> ports.SymbolMeta (name, signature, kind, line range, parent). Extension point for runtime .so loading (purego, not yet wired).

_Notes:_
Tree Center will be expanding this and working alongside with them. They great tool, really helps out the industry and what they do.

#### 3.2.7 File Watcher (`internal/adapters/fsnotify/`)
Recursive directory monitoring with debouncing. Filters .git, node_modules, vendor, etc. Implements ports.Watcher interface. Built and tested but NOT wired -- Watch() never called in app.go. No dynamic re-indexing pipeline connected.

_Notes:_
FS Notify, I'm assuming this is a file watcher, index watcher, for both their code base as well as session information. And so as long as it follows the rules of.getignore, I think we're pretty good. Where it doesn't have to follow it is in exceptions. And so we would have to create some way of allowing them to add exceptions outside of the.getignore. And I would let them do that manually somehow, as well as showing them in that same file of the ignorables where we're reading.

#### 3.2.8 Aho-Corasick Matcher (`internal/adapters/ahocorasick/`)
Multi-pattern string matching O(n+m+z). Implements ports.PatternMatcher interface. Currently stubbed -- tests skipped, implementation pending.

_Notes:_
I didn't even know we were using this. I think it's great. I think it's going to really help out with our security and our soon-to-be bitmask technique. I think it's going to work great there.

### 3.3 App Wiring (`internal/app/`)
Owns all components. Manages lifecycle (New -> Start -> Stop). Serializes learner access with mutex. Accumulates session metrics, tool metrics, conversation turns (ring buffer, 50 entries), activity entries (ring buffer). Implements socket.AppQueries. Handles onSessionEvent() and onSearchObserver().

_Notes:_
Make sure every code is commented, clean, well formatted, beautified. Make everyone want code like us.

### 3.4 Ports / Interfaces (`internal/ports/`)
Four interfaces: Storage, SessionReader, Watcher, PatternMatcher. Shared types: Index, LearnerState, SearchOptions, SessionEvent, TokenRef, SymbolMeta, FileMeta, DomainMeta, ToolEvent, TokenUsage. Agent-agnostic event model (7 event kinds).

_Notes:_
Great architecture, keep it up, keep clean code.

---

## 4. Data & Configuration

### 4.1 Atlas (`atlas/v1/`)
134 semantic domains across 15 focus areas. Embedded via //go:embed. Each domain has keywords and terms. Source of truth for domain enrichment.

_Notes:_


### 4.2 Key Paths
| Purpose | Path |
|---------|------|
| Database | `{ProjectRoot}/.aoa/aoa.db` |
| Status line | `{ProjectRoot}/.aoa/status.json` |
| Socket | `/tmp/aoa-{sha256(root)[:12]}.sock` |
| HTTP port | `{ProjectRoot}/.aoa/http.port` |
| Dashboard | `http://localhost:{port}` |
| Session logs | `~/.claude/projects/{encoded-path}/*.jsonl` |
| Daemon log | `{ProjectRoot}/.aoa/daemon.log` |
| Daemon PID | `{ProjectRoot}/.aoa/daemon.pid` |

_Notes:_


### 4.3 Hooks (`hooks/`)
Status line hook for Claude Code integration. Reads .aoa/status.json and displays in Claude Code status bar.

_Notes:_


---

## 5. Signal Flow & Data Pipeline

### 5.1 Session Ingestion Pipeline
```
Claude JSONL -> tailer (500ms poll) -> parser -> claude.Reader (Session Prism) -> app.onSessionEvent()
```
Events: UserInput, AIThinking, AIResponse, ToolInvocation, ToolResult, FileAccess, SystemMeta.

_Notes:_


### 5.2 Learning Signal Flow
```
UserInput     -> flush currentBuilder -> push user turn -> promptN++, bigrams, status line
AIThinking    -> bigrams + buffer thinking text on currentBuilder
AIResponse    -> bigrams + buffer response text + per-turn & global token accumulators
ToolInvocation -> range gate (0-500) -> file_hits -> observe
               -> TurnAction on currentBuilder
               -> activity ring buffer
```

_Notes:_


### 5.3 Search Signal Flow
```
Search (CLI) -> searchObserver -> signalCollector:
  1. query tokens -> keywords -> terms -> domains
  2. top 10 hit domains
  3. hit tags as terms -> domains
  4. hit content/symbol keywords -> terms -> domains
  -> all fed to learner observe()
  -> activity ring buffer entry
```

_Notes:_


### 5.4 Autotune Cycle
Every 50 prompts: 21-step algorithm (lifecycle management, curation, decay via 0.90 rate, pruning below 0.3 floor, competitive displacement to top 24 core). Writes status line.

_Notes:_


---

## 6. Tests & Quality

### 6.1 Unit Tests
Colocated with source (*_test.go in same package). Cover: tokenizer, search modes, learner observe/autotune/bigrams/dedup, enricher lookup, status generation, parser, tailer.

_Notes:_


### 6.2 Parity Tests (`test/parity_test.go`)
Load fixtures from test/fixtures/. Assert exact match against Python behavioral output. 26 search queries against 13-file index. 5 learner state snapshots. Zero tolerance for divergence.

_Notes:_


### 6.3 Integration Tests (`test/integration/cli_test.go`)
42 tests. End-to-end CLI commands. Daemon start/stop, search via socket, init, health, config, wipe.

_Notes:_


### 6.4 Activity Rubric Tests (`internal/app/activity_test.go`)
30 tests covering all action/source/attrib/impact combinations. searchAttrib (4), searchTarget (2), source casing, impact default, guided savings, no-savings, path stripping (4), bash filtering (3), full rubric (13), readSavings (4), ring buffer.

_Notes:_


### 6.5 Benchmarks (`test/benchmark_test.go`)
Search latency, autotune latency, startup time, memory footprint.

_Notes:_


### 6.6 Fixtures (`test/fixtures/`)
- `search/`: 26 queries, 13-file index state
- `learner/`: 5 state snapshots, 3 event streams
- `observe/`: Signal data for learner testing
- `SPEC.md`: Behavioral specification

_Notes:_


---

## 7. Known Gaps & Open Items

### 7.1 File Watcher Not Wired
fsnotify adapter built and tested. treesitter parser built and tested. But the pipeline (file change -> re-parse -> update index) is not connected in app.go. Watch() is never called.

_Notes:_


### 7.2 bbolt Lock Contention
`aoa init` fails while daemon holds the bbolt lock. Need an in-process reindex command so daemon can re-index without releasing its lock.

_Notes:_


### 7.3 Aho-Corasick Stubbed
ports.PatternMatcher interface exists. Adapter directory exists. Tests skipped, no implementation.

_Notes:_


### 7.4 Attribution Table Gaps
Items 8-11, 14-15 in the attribution table (Write/Edit=productive, Glob=unguided, Grep token cost, Autotune event, Learn event). See AT-01 through AT-08 on the board.

_Notes:_


### 7.5 `--invert-match` / `-v` Flag
Listed in flag parity table as "Not implemented." All other grep/egrep flags are done.

_Notes:_


### 7.6 Purego Grammar Loader
R-01 on the board. Runtime .so loading for tree-sitter grammars (alternative to 28 compiled-in CGo grammars). Not yet built.

_Notes:_


---

## 8. Future Phases

### 8.1 Phase 9 — Migration & Validation
Parallel run on 5 projects, search diff (100 queries), learner state diff (200 intents), benchmark comparison, migration docs.

_Notes:_


### 8.2 Phase 10 — Distribution
Purego .so loader, grammar downloader CI, goreleaser (linux/darwin x amd64/arm64), installation docs.

_Notes:_


### 8.3 v2 — Dimensional Analysis (Post-Release)
Multi-dimensional static analysis (security, performance, standards). YAML schema, dimension compiler, analyzer domain, query support. Deferred.

_Notes:_
