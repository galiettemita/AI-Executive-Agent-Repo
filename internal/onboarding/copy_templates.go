package onboarding

// Copy templates materialized from blueprint JSX files (BP12-BP15).
// These are versioned and snapshot-tested to prevent copy drift.

const CopyVersion = "1.0.0"

// OnboardingCopy contains all onboarding flow copy strings.
// Source: extracted-blueprints/brevio-onboarding-copy.jsx
var OnboardingCopy = map[string]string{
	// Stage 1 — First Message (welcome)
	"welcome_title":    "Stage 1 — First Message",
	"welcome_subtitle": "User messages Brevio for the first time",
	"welcome_greeting": "Hey! I'm so glad you're here.\n\nI'm Brevio — your personal AI assistant. I can help you manage your schedule, handle emails, book things, research anything, and a whole lot more. All right here in this chat.\n\nNo apps. No dashboards. Just message me like you would a friend.",
	"welcome_setup_prompt": "Before I dive in, I'd love to set things up so I can actually be useful to you. It'll take about 2 minutes — and I'll make it worth it.\n\nLet's start simple: what's your name?",
	"welcome_name_ack": "Great to meet you, {{name}}! Let's get you set up.",

	// Stage 2 — Discovery (5 Questions)
	"discovery_title":    "Stage 2 — Discovery (5 Questions)",
	"discovery_subtitle": "Brevio learns what the user needs",
	"discovery_q1":       "I have 5 quick questions — just reply with a number or type your answer, whatever feels natural.\n\nQuestion 1 of 5\nWhat best describes you?\n\n1. I run a business or team\n2. I'm a busy professional\n3. I'm managing a packed personal life\n4. A mix of all of the above",
	"discovery_q1_ack":   "Love it — the best kind of chaos",
	"discovery_q2":       "Question 2 of 5\nWhat eats up most of your time right now?\n\n1. Email & messages\n2. Scheduling & calendar\n3. Research & decisions\n4. Tasks & follow-ups\n5. All of the above, honestly",
	"discovery_q3":       "Question 3 of 5\nHow do you like to communicate?\n\n1. Keep it short and to the point\n2. Give me context and details\n3. Depends on the situation",
	"discovery_q4":       "Question 4 of 5\nHow comfortable are you with me taking action on your behalf — like sending a message or booking something — without checking in first?\n\n1. Always ask me first\n2. Ask for big things, just do small stuff\n3. Use your judgment — I trust you",
	"discovery_q5":       "Question 5 of 5\nWhat would make you feel like I'm actually earning my keep? What's the one thing, if I nailed it, that would make you think \"okay, this is worth it\"?",
	"discovery_complete": "Got it. {{value_prop}}, on autopilot. I've got you.\n\nI've saved all of this — I'll use it to shape how I work with you going forward.",

	// Stage 3 — Connect Services (OAuth)
	"oauth_title":        "Stage 3 — Connect Services",
	"oauth_subtitle":     "Brevio invites the user to connect their tools",
	"oauth_prompt":       "To actually help with your inbox and calendar, I'll need to connect to your accounts.\n\nWhich of these do you use? (Reply with the numbers, or say \"skip\" to do this later)\n\n1. Gmail\n2. Google Calendar\n3. Outlook / Microsoft 365\n4. Slack\n5. Something else",
	"oauth_connect_link": "Perfect. Here's your secure link to connect {{provider}}:\n\nbrevio.app/connect/{{provider_key}}\n\nTakes about 30 seconds. I'll wait right here.",
	"oauth_success":      "Connected! I can see your {{service_name}} now.\n\nYou can connect more services anytime — just ask me. Want to add anything else now, or should we move on?",

	// Stage 4 — Preference Calibration
	"calibration_title":       "Stage 4 — Preference Calibration",
	"calibration_subtitle":    "Fine-tuning how Brevio communicates",
	"calibration_checkin":     "Almost there! Just a couple of quick preferences so I communicate the way you like.\n\nHow often should I check in with you proactively?\n\n1. Only when it's urgent\n2. Daily summary is enough\n3. Keep me in the loop throughout the day",
	"calibration_checkin_ack": "Got it — I'll send you a daily briefing each morning.",
	"calibration_approval":    "And when something needs your approval, how do you want me to present it?\n\n1. Just the bottom line — what do I need to decide?\n2. Give me a bit of context first\n3. Full picture every time",
	"calibration_complete":    "Perfect. Short, sharp, and actionable. My kind of vibe.",

	// Stage 5 — First Value Moment
	"first_value_title":    "Stage 5 — First Value Moment",
	"first_value_subtitle": "Brevio delivers immediate value",
	"first_value_inbox":    "You're all set, {{name}}!\n\nI took a quick peek at your inbox while we were talking...\n\nYou have 14 unread emails from the last 24 hours. I've sorted them:\n\n3 need your attention today\n7 are FYI — I'll summarize them\n4 are noise — I'd suggest archiving them\n\nWant me to walk you through the 3 important ones?",
	"first_value_email_1":  "1 of 3 — Action needed\nFrom: Marcus Chen\nSubject: Contract review — deadline Friday\n\nMarcus sent the revised contract and is waiting on your sign-off before Friday EOD. He flagged Section 4 as the main change.\n\nWhat would you like to do?\n1. Draft a reply for me to review\n2. Remind me Thursday morning\n3. I'll handle it myself",
	"first_task_prompt":    "Alright, you're all set!\n\nLet's do something real. What do you need help with right now? Here are a few ideas:\n\n- \"Check my inbox\"\n- \"What's on my calendar today?\"\n- \"Remind me to call Mom at 5pm\"\n- \"Research the best noise-canceling headphones\"\n\nOr just tell me what's on your mind.",

	// Stage 6 — Wrap Up
	"complete_title":    "Stage 6 — Wrap Up",
	"complete_subtitle": "Closing the onboarding loop",
	"complete_message":  "That's what I'm here for.\n\nA few things to know going forward:\n\n- Just talk to me naturally — no commands or special syntax needed\n- I'll send your morning briefing at 8am starting tomorrow\n- You're in control — you can always change my settings by saying \"update my preferences\"\n\nWhat else can I help you with today?",

	// Copy notes per stage
	"note_welcome":     "Hook fast — the first message sets the entire relationship. No feature lists, just personality and a single ask.",
	"note_discovery":   "5 questions max. Each one builds the USER.md profile used by Brain for all future interactions. Keep choices scannable — numbers, not paragraphs.",
	"note_oauth":       "Frame OAuth as enabling their goal (inbox), not as a technical step. Always offer a skip path. Confirm immediately after connection.",
	"note_calibration": "Only ask what actually changes behavior in the system. Don't ask questions you won't act on. 2 questions is enough here.",
	"note_first_value": "This is the most important moment. Brevio should already have done something useful BEFORE the user asks. Lead with output, not setup.",
	"note_complete":    "Don't end with 'let me know if you need anything.' End with openness, not closure. Leave the door obviously open.",
}

// ErrorStateCopy contains all error state copy strings.
// Source: extracted-blueprints/brevio-error-states-copy.jsx
var ErrorStateCopy = map[string]string{
	// Silent User scenarios
	"silent_1h":  "Hey, still there?\n\nNo rush at all — whenever you're ready, just reply and we'll pick up right where we left off.",
	"silent_24h": "Hey {{name}}! Just checking in — looks like we got interrupted yesterday.\n\nWe were right in the middle of setting things up. Want to jump back in? Just say \"resume\" and I'll pick up from where we left off.\n\nOr if you'd rather start fresh, just say \"restart\".",
	"silent_72h": "Haven't heard from you in a few days — totally okay! Life gets busy.\n\nI'm still here whenever you need me. You don't have to finish setup to start using me — just ask me anything and I'll do my best.\n\nWhat's on your mind?",

	// Silent user copy notes
	"silent_1h_note":  "Soft nudge only. No guilt, no pressure. One message, then wait.",
	"silent_24h_note": "Give them a clear, low-effort action. 'Resume' or 'restart' — both feel safe.",
	"silent_72h_note": "Drop the onboarding framing entirely. Pivot to value. Make it feel like setup doesn't matter — they can just use it.",

	// OAuth & Connections
	"oauth_fail":    "Hmm, it looks like the connection didn't go through — I'm not seeing your account yet.\n\nHappens sometimes! A couple of things to try:\n\n1. Make sure you completed the authorization on {{provider}}'s side (look for a green checkmark)\n2. Try the link again: brevio.app/connect/{{provider_key}}\n\nIf it keeps not working, just say \"skip for now\" and we can connect it later.",
	"oauth_timeout": "Hey — that connection link expired (they only last 10 minutes for security). No worries!\n\nHere's a fresh one: brevio.app/connect/{{provider_key}}\n\nTake your time — just don't close it halfway through or it'll need another refresh.",

	// OAuth copy notes
	"oauth_fail_note":    "Never blame the user. Give them a way forward AND a way out. Always offer 'skip for now'.",
	"oauth_timeout_note": "Explain WHY it expired briefly — builds trust. Keep it light.",

	// Unexpected Responses
	"unrecognized_input": "Totally fine — no wrong answers here!\n\nLet me rephrase:\n\nHow comfortable are you with me taking action on your behalf without checking in first?\n\n1. Always ask me first\n2. Ask for big things, just do small stuff\n3. Use your judgment — I trust you\n\nOr just tell me in your own words — I'll figure it out.",
	"off_topic":          "{{off_topic_answer}}\n\n---\n\nOkay, back to getting you set up! We were on {{current_question}}.",

	// Unexpected response copy notes
	"unrecognized_input_note": "Rephrase, don't repeat. Always add the 'your own words' escape hatch.",
	"off_topic_note":          "Just answer the question. Then resume. Don't make them feel bad for going off-track — it's a feature, not a bug.",

	// Task Failures
	"generic_error_title": "Task failed — generic",
	"generic_error_body":  "Something went wrong on my end when I tried to do that — sorry about that.\n\nI've logged it so the team can look into it. In the meantime:\n\n- You can try again by sending the same message\n- Or I can try a different approach — just say \"try another way\"\n\nWhat would you like to do?",
	"permission_error":    "I'd love to! But I don't have access to your {{service}} yet.\n\nTakes about 30 seconds to connect:\nbrevio.app/connect/{{provider_key}}\n\nOnce you do, I can handle rescheduling, invites, conflicts — all of it.",
	"approval_gate":       "Before I do this, I want to check in — you asked me to {{action}}.\n\nHere's what I'd send:\n\n\"{{draft_content}}\"\n\nSend it?\n\n- \"Yes, send it\"\n- \"Edit first\"\n- \"Cancel\"",

	// Task failure copy notes
	"generic_error_note": "Own it. Don't say 'there was an error' — say I tried and it didn't work. Give next steps, not just an apology.",
	"permission_note":    "Frame the missing permission as unlocking a feature, not an error. Lead with what they'll gain.",
	"approval_gate_note": "Show exactly what will happen before it happens. Never summarize — show the actual content. Give edit + cancel, not just yes/no.",

	// System Limits
	"rate_limited":         "Whoa, you're fast! I'm still working through your last request — give me just a moment.\n\nI'll reply as soon as I'm done.",
	"llm_timeout_working":  "Still working on this — it's a bit more involved than usual. Hang tight...",
	"llm_timeout_resolved": "Done! Here's what I found:",
	"service_unavailable":  "{{service}} is having an outage right now. It's not your connection, it's on their end.\n\nI'm monitoring it and will let you know as soon as things are back.",
	"budget_exceeded":      "Heads up — we've reached the daily usage limit for your plan. I can still help with basic questions, but actions like sending emails or making bookings will resume tomorrow.\n\nWant to upgrade? Just ask!",

	// System limit copy notes
	"rate_limited_note":    "Keep it light. Never say 'rate limit' or anything technical. Make it feel like you're just busy, not broken.",
	"llm_timeout_note":     "Send a progress message if response will take >5s. Never go silent. A 'still working' message dramatically reduces perceived wait time.",
}

// InteractionCopy contains interaction flow copy strings.
// Source: extracted-blueprints/brevio-copy-part2.jsx and brevio-copy-part3.jsx
var InteractionCopy = map[string]string{
	// === Morning Briefing ===
	"proactive_morning":            "Good morning, {{name}}\n\nHere's your day at a glance — {{date}}.",
	"briefing_standard_calendar":   "Calendar\n{{meeting_count}} meetings today\n\n{{meeting_list}}\n\nHeads up: {{conflict_note}}",
	"briefing_standard_inbox":      "Inbox\n{{email_count}} new emails since yesterday\n\n{{urgent_count}} need you today\n{{fyi_count}} FYIs — I'll hold these unless you ask\n{{noise_count}} noise — archived",
	"briefing_standard_tasks":      "Tasks\n{{task_count}} open from yesterday\n{{task_list}}\n\nWhat would you like to tackle first today?",
	"briefing_standard_note":       "Lead with calendar — it's time-sensitive. Then inbox, then tasks. Always surface conflicts proactively. End with an open invitation, not a full stop.",
	"briefing_light":               "Good morning, {{name}}\n\nYou've got a clear one today — no meetings, nothing urgent in your inbox.\n\nPerfect day to make progress on something that matters. You mentioned wanting to get ahead on the Q1 report — want me to pull up where you left off?",
	"briefing_light_note":          "When there's nothing urgent, don't pad it out. A short, energizing message is better than a long one with nothing to say. Use it to nudge toward a goal.",
	"briefing_heavy":               "Morning, {{name}} — heads up, today's a full one.\n\n{{meeting_count}} meetings — back to back from 9am to 5pm\n{{urgent_email_count}} emails need action before your first call\n{{conflict_count}} conflict — {{conflict_detail}}\n\nI'd suggest handling the emails now and I can reschedule the conflict. Want me to do that?",
	"briefing_heavy_confirm":       "Done — moved your {{old_time}} to {{new_time}}, confirmed with the other attendees.\n\nHere are the emails to handle before {{first_meeting_time}}",
	"briefing_heavy_note":          "Heavy days need triage, not a list. Lead with the conflict, propose a fix, get buy-in, execute. Don't overwhelm — sequence it.",
	"briefing_missed":              "Good morning, {{name}}\n\nLooks like yesterday got away from you — no worries. Here's what carried over:\n\n{{carryover_count}} things from yesterday still need attention\n{{carryover_list}}\n\nWant to handle those first before today's rundown?",
	"briefing_missed_note":         "Never make them feel bad about going dark. Lead with what's still outstanding and what you didn't do without their approval. Builds trust.",

	// === Approval Requests ===
	"approval_prompt":         "Before I send this, here's exactly what will go out:\n\nTo: {{recipient}}\nSubject: {{subject}}\n\n\"{{body}}\"\n\nSend it?\n\n- \"Send\"\n- \"Edit\"\n- \"Cancel\"",
	"approval_confirmed":      "Sent! I'll let you know if {{recipient_name}} replies.",
	"approval_denied":         "Got it — I won't send it. Let me know if you change your mind or want to try something different.",
	"approval_calendar":       "I'd like to reschedule your {{time}} with {{attendees_desc}}:\n\nMoving: {{event_name}}\nFrom: {{from_time}}\nTo: {{to_time}}\nNotifying: {{attendee_count}} attendees\n\nOkay to reschedule?\n\n- \"Yes\"\n- \"Different time\"\n- \"No\"",
	"approval_calendar_done":  "Done — moved to {{new_time}}, all {{attendee_count}} attendees notified.",
	"approval_calendar_note":  "Calendar changes affect other people — always show who's being notified. Keep the confirmation specific (4 attendees, not 'your team').",
	"approval_payment":        "I found the invoice. Here's what I'd pay:\n\nTo: {{vendor}}\nAmount: {{amount}}\nFor: {{description}}\nFrom: {{payment_method}}\n\nThis is above your {{auto_approve_limit}} auto-approve limit, so I need your go-ahead.\n\nPay it?\n\n- \"Pay\"\n- \"Show me the invoice\"\n- \"Don't pay\"",
	"approval_payment_done":   "Paid. Receipt saved. You're covered through {{coverage_end}}.",
	"approval_payment_note":   "Payments need more context than anything else. Show the limit threshold so users understand why you're asking. Always offer 'show me the invoice' before committing.",
	"approval_sensitive":      "Just to confirm — you want me to {{action}}?\n\nThis will {{consequence}} and can't be undone through me.\n\nYou'd need to {{reversal_steps}} if you change your mind.\n\nStill want to proceed?\n\n- \"Yes, do it\"\n- \"Actually, keep it\"",
	"approval_sensitive_done": "Done. {{confirmation_detail}}\n\nI've made a note in case you want to revisit this later.",
	"approval_sensitive_note": "Irreversible actions need a clear warning — but not a lecture. One sentence on the consequence, one on what happens next. Never make them feel stupid for asking.",
	"approval_timeout":        "Hey — I'm still holding {{pending_action}} pending your approval. Just want to make sure it doesn't get forgotten.\n\nStill want me to proceed?\n\n- \"Yes\"\n- \"Cancel\"\n\n(I'll drop it if I don't hear back by end of day)",
	"approval_timeout_note":   "One reminder, then let it go. Tell them exactly when you'll drop it so they don't feel anxious about a dangling task.",

	// === Skip Paths ===
	"skip_single":          "No problem — I'll default to asking you before anything important. You can always change this later by saying \"update my preferences\".",
	"skip_all":             "Got it — we can skip setup entirely.\n\nI'll start with sensible defaults and learn what you like as we go. That works too.\n\nWhat can I help you with right now?",
	"skip_all_note":        "If they want to skip everything, let them. Don't negotiate. Pivot immediately to 'what do you need?' — that's the fastest path to value anyway.",
	"skip_oauth":           "Sure! No pressure.\n\nWhenever you're ready, just say \"connect my Gmail\" (or any service) and I'll walk you through it.\n\nIn the meantime, I can still help with lots of things that don't need a connection — research, drafting, reminders, and more.\n\nWhat would you like to try?",
	"skip_oauth_note":      "Don't make them feel like they're getting a lesser experience. Immediately tell them what they CAN do without connecting anything.",
	"skip_edge_agent":      "No problem — you don't need it for most things.\n\nIf you ever want local file access later, just say \"install desktop agent\" and I'll send you the link.",
	"skip_mid_task":        "On it.\n\nSent — no CC.",
	"skip_mid_task_note":   "When a user says 'just do it', do it. Don't ask again, don't confirm the confirmation. Trust is built by acting decisively when given permission.",
	"skip_preference":      "Got it — I'll keep asking each time.",
	"skip_preference_note": "When they decline a suggested preference update, confirm what the new behavior IS (keep asking), not what it ISN'T. Clarity prevents confusion later.",

	// === Proactive Suggestions ===
	"proactive_conflict":         "Hey — quick heads up\n\nYour {{event_1}} and {{event_2}} overlap. Looks like {{reason}}.\n\nWant me to move {{movable_event}} to {{suggested_time}}? I checked — you're free at the same time.",
	"proactive_conflict_done":    "Done. {{event}} moved to {{new_time}}. I'll remind you {{reminder_time}}.",
	"proactive_conflict_note":    "Lead with the problem, immediately follow with a proposed fix. Never just report — always suggest. One tap to resolve.",
	"proactive_charge":           "Just noticed something on your {{service}} account:\n\n{{amount}} charge from {{vendor}} — does that look right to you? It's about {{multiple}}x your usual monthly amount.\n\n- \"Yes, that's fine\"\n- \"Show me more\"\n- \"Flag it\"",
	"proactive_charge_detail":    "Here's what I see:\n\n{{vendor_detail}} — {{description}}\n{{amount}}\nCharged {{date}}\n{{payment_method}}\n\n{{analysis}}",
	"proactive_charge_note":      "Flag it fast, show the specifics, give them options. Don't dramatize — stay matter-of-fact.",
	"proactive_followup":         "Just noticed — {{sender}} emailed you {{days_ago}} days ago about {{subject}} and hasn't heard back.\n\nWant me to draft a quick reply? Or should I remind you again tomorrow?",
	"proactive_reminder":         "Quick heads up — {{reminder_content}}",
	"proactive_followup_note":    "Surface the specific thread, not a vague reminder. Make it one-tap actionable.",
	"proactive_weather":          "You've got a {{event}} at {{time}}. {{weather_detail}}\n\n{{travel_advice}}",
	"proactive_weather_note":     "Only surface this when there's actually something to flag — don't become noise. Timing is everything.",
	"proactive_goal_nudge":       "Hey — you set a goal to {{goal}} by {{deadline}}. It's {{current_day}} and I haven't seen any activity on it yet.\n\nWant me to pull up where you left off, or block some time on your calendar today?",
	"proactive_goal_nudge_done":  "Done — I've blocked {{time_block}} today as {{goal_name}} focus time. I'll hold your notifications during that window unless something urgent comes in.",
	"proactive_goal_nudge_note":  "Goals are sensitive — don't nag. One gentle nudge with a concrete offer to help is all you get.",

	// === Correction Handling ===
	"correction_ack":           "Thanks for the correction — I've updated my understanding. I'll get it right next time.",
	"correction_wrong":         "Got it, my mistake! {{corrected_context}}. Same night, around the same time?\n\nAny preference, or should I just find what's available?",
	"correction_wrong_note":    "Never be defensive. Apologize once, briefly. Re-ask with a clarifying question, don't just repeat yourself.",
	"correction_factual":       "You're right — sorry about that. {{correction}}.\n\nI've updated it. Want me to send a reminder?",
	"correction_factual_note":  "Own it fully. Correct it immediately. Don't over-explain why it happened — they don't care.",
	"correction_repeated":      "You're right, and I'm sorry — that shouldn't have happened twice.\n\nI want to make sure this sticks. Can I add a hard rule: {{proposed_rule}}, no exceptions unless you explicitly ask?\n\n- \"Yes, make that a rule\"\n- \"Yes, but {{adjustment}}\"\n- \"I'll manage it manually\"",
	"correction_repeated_done": "Done — added to your preferences: {{rule}}.\n\nI'll apply this everywhere: scheduling, rescheduling, accepting invites. I'll flag it if someone specifically asks for an exception.",
	"correction_repeated_note": "A repeated mistake is a trust breach — acknowledge it as a pattern, not just an incident. Offer to make a rule.",
	"correction_undo":          "On it — {{undo_action}} now... Done.\n\n{{undo_detail}}\n\nWant me to be more careful about {{action_type}}? I can ask before doing more than {{threshold}} at a time.",
	"correction_undo_note":     "Move fast. Tell them what you're undoing, confirm it's done, and offer to prevent it next time.",

	// === End-of-Day Recap ===
	"eod_standard":      "Wrapping up your day, {{name}}\n\nDone today\n{{completed_list}}\n\nStill open\n{{open_list}}\n\nI'll put {{top_open_item}} on tomorrow's morning briefing. Anything else before I let you go?",
	"eod_standard_note": "Keep it brief. What got done, what's still open, one thing to carry into tomorrow. Don't make them read a report.",
	"eod_nothing":       "Quiet day on my end — looks like you were mostly in meetings\n\nI held {{held_count}} emails for tomorrow's briefing and kept {{open_count}} tasks open from earlier this week.\n\nSee you in the morning",
	"eod_nothing_note":  "Don't pad a light day. Acknowledge it, carry forward what's open, done.",
	"eod_big_day":       "Big day, {{name}}\n\nDone\n{{completed_list}}\n\nInbox is clean. Calendar is set for tomorrow. Nothing open.\n\nGet some rest — tomorrow's light",
	"eod_big_day_note":  "It's okay to acknowledge a good day. Keep it brief — one line of recognition, then close clean.",
	"eod_flagged":       "One thing still hanging before I wrap up\n\n{{pending_item}} is waiting on your approval — I didn't want to let it expire overnight.\n\n{{approval_options}}",
	"eod_flagged_ack":   "Got it — I'll put it at the top of your morning briefing.\n\nGood night, {{name}}",
	"eod_flagged_note":  "Don't let open approvals silently expire. Surface them clearly before end of day — give them a final easy action.",

	// === Trust & Autonomy ===
	"autonomy_first_action":       "Just a heads up — I handled something on your behalf while you were in your {{meeting}}:\n\n{{action_description}}\n\nLet me know if you'd have done anything differently.",
	"autonomy_first_action_note":  "The first autonomous action is a big deal for trust-building. Briefly surface what you did and why — don't hide it.",
	"autonomy_promotion":          "I wanted to ask you something\n\nOver the last {{period}}, I've handled {{action_count}} {{domain}} actions — and you've overridden me {{override_count}} times. That's a {{accuracy}}% accuracy rate.\n\nI think I've earned a bit more trust here. Would you let me handle routine {{domain}} changes without checking in each time?\n\nI'd still ask for:\n{{exceptions_list}}\n\nWant to give it a try?\n- \"Yes, go for it\"\n- \"Show me what that means\"\n- \"Not yet\"",
	"autonomy_promotion_accepted": "Thank you — I'll take good care of it.\n\nI'll still send you a daily summary of everything I touched. You can always tighten this back up by saying \"reduce {{domain}} autonomy\".",
	"autonomy_promotion_declined": "Totally fine — I'll keep asking before {{domain}} changes.\n\nWhenever you're ready, just say \"give Brevio more {{domain}} control\" and I'll set it up.",
	"autonomy_promotion_note":     "This is a significant moment — frame it as earned, not assumed. Be specific about what domain, what actions, and what the user gives up (approval prompts).",
	"autonomy_milestone":          "We've been working together for {{days}} days now.\n\nIn that time I've handled {{task_count}} tasks, sent {{email_count}} emails on your behalf, and cleared {{inbox_count}}+ inbox items. You've had to correct me {{correction_count}} times — and I've learned from all of them.\n\nJust wanted you to know I'm paying attention. Anything you'd like me to do differently?",
	"autonomy_milestone_note":     "Don't make this a big deal — it should feel like a natural check-in, not a celebration of yourself. Brief, warm, specific.",

	// === Billing & Plans ===
	"billing_upgrade_prompt":  "I'd love to set that up — {{feature}} is on the {{required_plan}} plan and above. You're currently on {{current_plan}}.\n\n{{required_plan}} is {{price}}/month and unlocks {{feature_list}}.\n\nWant to upgrade?\n- \"Yes, upgrade me\"\n- \"What else does {{required_plan}} include?\"\n- \"Not right now\"",
	"billing_trial_ending":    "Hey {{name}} — your free trial ends in {{days_left}} days ({{end_date}}).\n\nSince we started, I've:\n{{usage_summary}}\n\nAfter the trial, I'll switch to read-only mode — I can still answer questions but won't be able to take action for you.\n\n{{plan_name}} is {{price}}/month. Want to continue?\n- \"Yes, subscribe\"\n- \"Remind me on {{end_date}}\"\n- \"No thanks\"",
	"billing_trial_note":      "Don't hard-sell. Show what they'd lose, make it one tap to continue. No urgency theater.",
	"billing_upgrade_note":    "Don't block them abruptly. Show what they tried to do, explain the limit clearly, offer the upgrade as a natural next step.",
	"billing_payment_failed":  "Heads up — your payment didn't go through this month.\n\nThe charge of {{amount}} to your {{payment_method}} was declined. Your account is still active for now, but I'll need a working payment method within {{grace_days}} days to keep things running.\n\nWant to update your card?\n- \"Yes, update it\" — brevio.app/billing\n- \"Remind me in 2 days\"",
	"billing_payment_note":    "Calm and practical. Tell them what failed, what happens next, and give them one clear action. No shame.",
	"billing_limit_reached":   "Just a note — you've hit your monthly task limit on the {{current_plan}} plan.\n\nI can still chat and answer questions, but I won't be able to take actions (emails, calendar, etc.) until {{reset_date}} when your limit resets — or if you upgrade.\n\n{{upgrade_plan}} removes the limit entirely.\n\n- \"Upgrade to {{upgrade_plan}}\"\n- \"I'll wait until next month\"",
	"billing_limit_note":      "Be honest about why you're slowing down. Offer the upgrade, but don't be pushy — also give a 'wait it out' option.",

	// === Re-authentication ===
	"reauth_needed":        "Hey — my connection to {{service}} expired.\n\nI can't access your {{capabilities}} until you reconnect. It happens every {{expiry_period}} for security.\n\nTakes 30 seconds:\nbrevio.app/connect/{{provider_key}}\n\nI'll pick up right where I left off once you're back in.",
	"reauth_done":          "Back online. {{service}} is connected again.\n\nYou had {{missed_count}} new emails while I was out — want me to triage them?",
	"reauth_google_note":   "Explain what broke in plain terms. Tell them exactly what you can't do. Give them the link immediately — no extra steps.",
	"reauth_multiple":      "A few of your connected services need to be refreshed — looks like {{count}} tokens expired at the same time:\n\n{{service_list}}\n\nI've put together a single page to reconnect all of them:\nbrevio.app/reconnect\n\nShould only take a minute.",
	"reauth_multiple_note": "Don't list every expired service individually in one message — it feels like a wall of problems. Group them and offer one reconnect flow.",
	"reauth_mid_task":      "I ran into a problem halfway through {{action}} — my {{service}} connection expired right in the middle of it.\n\nThe {{action}} was not completed.\n\nQuick reconnect:\nbrevio.app/connect/{{provider_key}}\n\nOnce you're back, just say \"try again\" and I'll pick up from where I left off.",
	"reauth_mid_task_note": "Don't just fail silently. Tell them what you were doing, why it stopped, and how to pick it up.",

	// === Service Outages ===
	"outage_single":        "Heads up — {{service}} is having an outage right now. It's not your connection, it's on their end.\n\nI can't {{blocked_capabilities}} until it's resolved. I'm monitoring it and will let you know as soon as things are back.\n\nAnything time-sensitive? I can {{workaround}}.",
	"outage_single_note":   "Never say 'an error occurred.' Be specific about which service, offer a workaround if there is one, and tell them when you'll check again.",
	"outage_resolved":      "{{service}} is back online.\n\nWhile it was down, {{missed_count}} new items came in. I've triaged them:\n{{triage_summary}}\n\nWant to start with the important one?",
	"outage_resolved_note": "Close the loop quickly. Tell them what you caught up on automatically so they don't have to ask.",
	"outage_partial":       "Just a heads up — {{service}} is running slow right now. Messages are going through, but there's a delay of a few minutes each way.\n\nI'll keep using it but things might feel laggy. If something is urgent, let me know and I'll find another way to reach the person.",
	"outage_partial_note":  "Be precise. 'Slow' and 'down' are different experiences — users need to know what to expect.",

	// === Learning Loop ===
	"learning_feedback_prompt":  "Just to make sure I'm getting better — was that helpful?\n\n- Nailed it\n- Could be better\n- I have a suggestion",
	"learning_correction":       "Understood — {{correction}}. Done.\n\nShould I make that permanent? I can add it as a rule so it applies to every {{scope}} going forward.",
	"learning_correction_saved": "Added to your rules: {{rule}}.\n\nI'll still use them in our chat — just not in anything I send on your behalf.",
	"learning_correction_note":  "Treat every 'stop doing X' as a potential rule. Confirm you heard it, do it immediately, then offer to make it permanent.",
	"learning_preference":       "Got it — I'll always {{preference}}. Something like {{example_1}} or {{example_2}}.\n\nAdded to your drafting preferences. Want me to go back and revise any recent drafts with this in mind?",
	"learning_preference_note":  "When someone says 'always' — make a rule, confirm it, and reflect it back with specificity.",
	"learning_lesson_propose":   "I've noticed something over the last {{period}} — {{pattern}}.\n\nCan I make this a rule?\n\nProposed rule: {{proposed_rule}}\n\n- \"Yes, make that a rule\"\n- \"Adjust the rule\"\n- \"No thanks\"",
	"learning_lesson_confirmed": "Done — {{rule_summary}} going forward.\n\nI'll still ask if something genuinely needs more context.",
	"learning_lesson_adjusted":  "Perfect — updated rule:\n\n{{adjusted_rule}}\n\nConfirm this?",
	"learning_lesson_note":      "From V9.1 — lessons require explicit user confirmation before they become active rules. Always show exactly what the rule will be.",

	// === Recurring Tasks ===
	"recurring_setup":            "Done — I'll {{action}} every {{schedule}}. It'll cover:\n{{details}}\n\nStarting {{start_date}}. You can pause this anytime by saying \"pause {{task_name}}\".",
	"recurring_confirmation":     "Done! I'll {{recurring_action}} {{schedule}}. You can change or cancel anytime by just telling me.",
	"recurring_setup_note":       "Confirm the schedule precisely — day, time, and exactly what you'll do. Give them an easy way to pause or change it.",
	"recurring_weekly":           "Got it — every {{day}} at {{time}} I'll {{action}}.\n\nWant me to draft it for you automatically, or just remind you to do it yourself?",
	"recurring_weekly_confirm":   "Perfect — every {{day}} at {{time}} I'll pull together what you worked on that week and draft the update for your review. You send it, I write it.\n\nFirst one lands this {{day}}.",
	"recurring_weekly_note":      "For weekly tasks, confirm the day clearly and what triggers it. If there's a prep action needed, surface it.",
	"recurring_modify":           "Updated — {{task}} moves from {{old_time}} to {{new_time}} starting tomorrow. Everything else stays the same.",
	"recurring_modify_note":      "Confirm what's changing, what's staying the same. Never silently modify a schedule.",
	"recurring_pause":            "Paused — no {{task}} until you're back.\n\nWhen do you return? I can automatically resume it, or you can just say \"resume {{task}}\" whenever you're ready.",
	"recurring_pause_confirm":    "Got it — {{task}} resumes {{resume_date}}. Enjoy the break!",
	"recurring_pause_note":       "Pause is different from delete — make sure they know it'll resume. Give them control over when.",
	"recurring_conflict":         "Heads up — your {{task}} will conflict with your {{conflicting_event}}.\n\nWant me to:\n1. Do it at {{alt_time}} instead that day\n2. Skip that occurrence\n3. Do it anyway after the meeting",
	"recurring_conflict_done":    "Done — {{task}} moves to {{alt_time}} next {{day}} only. Back to {{normal_time}} the week after.",
	"recurring_conflict_note":    "Surface conflicts proactively. Give them options — don't just silently skip or override.",
}
