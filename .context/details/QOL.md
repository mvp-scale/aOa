# QOL — Dashboard Quality of Life Feedback

Paste your comments after the `|` on any line. Leave blank = no issue.

---

## Global Chrome (all tabs)

### Nav Bar
- Brand mark (angle + "aOa" text)             |  
- Status badge ("LIVE"/"OFFLINE")              |
- Theme toggle (dark/light)                    |

### Footer
- Online indicator (dot + "Online"/"Offline")  |
- Version string ("v1.0.0")                    |
- Motto text                                   |
- Live clock (HH:MM:SS)                        | The live indication, we probably need to put something in here that syndicates that it's been refreshed. If anything on the page got changed, this should hopefully pulse to say data changed. I believe that was a universal format for only changing the text that changed, the data or the metric that changed. In this case, live is blinking, and I like it. But maybe we can give some kind of customer delight to let them know that data just recently changed.

---

## Tab 1: Live

### Hero Card
- Label ("ANGLE OF ATTACK")                    |
- Headline (random phrase)                     |
- Support line (burn rate, ctx, model, turn#)  |  We shouldn't show if the- if there's no data in the element, we shouldn't just show a blank line, just don't show it until we have data.

### Hero Metrics (4 cells)
- Cell 0: "tokens saved" (green)               |
- Cell 1: "cost saved" (green)                 | We imply cost saves, but it's based on API rates.  We need to say this in a very compact way.
- Cell 2: "time saved" (cyan)                  |
- Cell 3: "extended" (blue)                    | Let's have an offline conversation about extended. After we have corrected, we can put this in the backlog area.

### Stats Grid (6 cards)
- Context Util% (cyan)                         | You've had context utilization for some time. It's actually inside of the utilization usage.text. No, not that. Context? Yeah, context.json. You already have this and you've had context.json under the hooks for some time. This context utilization present shows this exactly what you would want. It's a percentage and it's already been pre-calculated for you.
- Burn Rate (red)                              | I like the burn rate, but we have to be more transparent to what this represents. It seems high, but it may be accurate. Let's talk about it.
- Guided Ratio (green)                         |
- Output Speed (blue)                          | I really like any metric that talks us tells us about tokens per second speed. I do think we have a potential way of measuring speed since we're capturing the entire conversation from Claude in session history. And we may in fact be able to get some relative average over time of showing the speed. I want to see this speed kind of in the last five minutes so it doesn't spike up and down. It has some level of consistency, but we don't need to create a long running average.
- Cache Hit% (purple)                          | Cash hit at 100%. We need to figure out why this is always 100%. I don't believe this to be accurate.
- Autotune (yellow)                            | 

### Activity & Impact Table
- Table header row                             |
- Time column                                  |
- Action column (colored pills)                |
- Source column                                |
- Attrib column (pills)                        |
- Impact column                                |
- Learned column                               |
- Target column                                |

---

## Tab 2: Recon

(Leaving for later)

---

## Tab 3: Intel

### Hero Card
- Label ("INTEL")                              |
- Headline (random phrase)                     |
- Support line (core, terms, keywords, hits)   |

### Hero Metrics (4 cells)
- Cell 0: "core domains" (purple)              |
- Cell 1: "domain velocity" (green)            | Let's have a conversation about what this truly means.
- Cell 2: "term coherence" (cyan)              |   Let's have a conversation about what this truly means.
- Cell 3: "KW->domain %" (yellow)              |  Let's have a conversation about what this truly means.  My concern is this doesn't this does not add value to the user. It's just data upon data. There's probably a better sell here. Let's figure out which ones answer the question on Intel better.

### Stats Grid (6 cards)                       |  

- Domains (purple)                             |
- Core (green)                                 |
- Terms (cyan)                                 |
- Learning Rate (blue)                         | Let's talk about learned rate. I think it's good, but I wonder if it's being expressed cleanly or intelligently. What does it truly mean? It raises some questions, and so let's find out what this what this could be.
- Bigrams (yellow)                             |
- Total Hits (red)                             |

### Domain Rankings Table
- Table header                                 | Title:  Domain Rankings — sorted by learning signal strength  24 domains ...  Think we can think of some better ways instead of saying sorted by learned signals. The idea is that it's intelligence-based, multi-factore, it's sorted by angles of intent, timing, knowledge, a few. And the idea is that all results are ranked. So how do we imply that? Showing 24 domains, there are hopefully a lot more of domains. And so I wonder the value of showing 24 domains here.
- # (rank) column                              |
- Domain column (@name)                        |
- Hits column (float, flash on change)         |
- Terms column (pills, glow on change)         |
- Footer text                                  |

### N-gram Metrics
- Bigrams section (cyan bars, top 10)          |
- Cohits KW->Term (green bars, top 5)          |
- Cohits Term->Domain (purple bars, top 5)     |

---

## Tab 4: Debrief

### Hero Card
- Label ("DEBRIEF")                            |
- Headline (random phrase)                     |
- Support line (turns, tokens, cache$, speed)  | Speed is blank. Blank elements should not be presented. There's no reason to show a blank.

### Hero Metrics (4 cells)
- Cell 0: "input tokens" (cyan)                |
- Cell 1: "output tokens" (green)              |
- Cell 2: "cache savings" (blue)               | We have to figure out how to calculate this as well as cash savings, cost savings, where it makes sense and it is understood by the users.
- Cell 3: "cost/turn" (purple)                 | We have to figure out how to calculate this as well as cash savings, cost savings, where it makes sense and it is understood by the users.

### Stats Grid (6 cards)
- Cache Hit% (blue)                            | Cash hit of 100% offers very little value. Let's understand what's actually happening here to understand Claude's cash hit strategy.  Perhaps it's not so much the cache hit rate as it is the size of the current cache. I wonder if we have access to that.
- Output Speed (green)                         | Output speed, I would love this. I know we have the capability of doing this, but it's a calculated tokens per second. We have from the time the user says something all the way through that particular flow. We could track all the tokens that are generated through prompt, think time, and assistant, at least through the conversational speed, not tool speed.
- Avg Turn Dur (cyan)                          | 
- Tool Density (yellow)                        |
- Amplification (purple)                       | The term's too vague, we need a description.
- Model Mix (red)                              |

### Conversation Feed (left column)
- Header + exchange count badge                |
- "Now" button                                 |
- Turn separator ("Turn N")                    |
- User message (role, tokens, time, text)      | Everything is wonderful. I really do like what you have here. I just want to make sure we're not truncated.
- Thinking block (role, count, tokens, text)   | Everything is wonderful. I really do like what you have here. I just want to make sure we're not truncated.
- Assistant message (role, model, tokens, text)| Everything is wonderful. I really do like what you have here. I just want to make sure we're not truncated. 
- "NOW" marker                                 |

### Actions (right column)
- Header                                       |
- Group header ("Turn N", Save, Tok)           |
- Tool chip (color by type)                    | We need to do an audit of all possible tools and how they should be responding. Some we can, some we can't, but we should definitely have a response for knowing what we can do and what offers value of little work for us.
- Path (truncated 80 chars)                    | Attempt to show the full path wherever possible within the space provided.
- Savings cell (down-arrow %)                  |
- Token count (green/red/dim)                  |
- "NOW" marker                                 |

---

## Tab 5: Arsenal

### Hero Card
- Label ("ARSENAL")                            |
- Headline (random phrase)                     |
- Support line (sessions, saved, time, ROI)    |

### Hero Metrics (4 cells)
- Cell 0: "cost avoidance" (green)             |
- Cell 1: "sessions extended" (blue)           | Again, we need to be able to calculate if we're estimating how long your session is, we need input, and we need to show the user how to do it. If we can calculate it based on them telling us when their session ends, then great.
- Cell 2: "cache savings" (cyan)               | This seems really high in cash savings. We have to articulate this so we don't lose confidence.
- Cell 3: "efficiency" (purple)                |

### Stats Grid (6 cards)
- Guided Ratio (cyan)                          | 
- Unguided Cost (red)                          |
- Sessions (blue)                              |
- Avg Prompts (purple)                         |
- Tokens Saved (green)                         |
- Read Velocity (yellow)                       | Let's understand what read velocity is. Is it more tool velocity? I'm trying to understand.

### Daily Savings Chart
- Header + total saved badge                   |
- Bar chart (date groups)                      | This needs to be more compact. Uh a day should be rolled up and you should be able to fit probably the rolling two week view here and it should be evenly spaced and it shouldn't have big gaps between the days.
- Actual bars (green)                          | This needs to be more compact. Uh a day should be rolled up and you should be able to fit probably the rolling two week view here and it should be evenly spaced and it shouldn't have big gaps between the days.
- Counterfactual bars (red)                    | This needs to be more compact. Uh a day should be rolled up and you should be able to fit probably the rolling two week view here and it should be evenly spaced and it shouldn't have big gaps between the days.
- Legend                                        |

### Session History Table
- Header + session count badge                 |
- ID column (first 8 chars)                    |
- Date column                                  |
- Dur column                                   |
- P (prompts) column                           |
- R (reads) column                             |
- Guided column (% + inline bar)               | There seems to be an inline bar. Let's have a conversation on what the intent was here. It's mostly blank.
- Saved column                                 |
- Time column                                  |
- Waste column                                 | We're gonna improve this.  Give me your thoughts, what we can add, uh how we can track more waste.
- R/P column                                   | R/P, it's undefined. The user would be confused. We need to understand exactly what is being said here.

### Learning Curve Chart
- Canvas chart (guided ratio over sessions)    | I love the learning curve, but we need to define it. I think there's an opportunity for some true excellence here. Maybe an overlay between learning and cost and trying to show how, over the last thirty days, we have increased savings. 
- X-axis labels (S1, S2...)                    | I  love the learning curve, but we need to define it. I think there's an opportunity for some true excellence here. Maybe an overlay between learning and cost and trying to show how, over the last thirty days, we have increased savings.
- Y-axis (0-1)                                 | I love the learning curve, but we need to define it. I think there's an opportunity for some true excellence here. Maybe an overlay between learning and cost and trying to show how, over the last thirty days, we have increased savings.
- "Need 2+ sessions" message                   | I love the learning curve, but we need to define it. I think there's an opportunity for some true excellence here. Maybe an overlay between learning and cost and trying to show how, over the last thirty days, we have increased savings.

### System Status
- PID                                          |
- Uptime                                       |
- DB                                           |
- Files                                        |
- Tokens                                       |
