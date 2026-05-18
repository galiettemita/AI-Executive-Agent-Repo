import { useState } from "react";

const allSections = [
  {
    id: "proactive",
    label: "Proactive Suggestions",
    icon: "⚡",
    color: "#F7C94F",
    description: "Mid-day, unprompted nudges",
    scenarios: [
      {
        id: "pro_conflict",
        label: "Scheduling conflict detected",
        description: "Brevio spots a calendar clash before the user does",
        note: "Lead with the problem, immediately follow with a proposed fix. Never just report — always suggest. One tap to resolve.",
        messages: [
          { from: "brevio", text: "Hey — quick heads up 👋\n\nYour 2pm investor call and 2:30pm dentist appointment overlap. Looks like the call was just extended to 3pm.\n\nWant me to move the dentist to Thursday? I checked — you're free at the same time." },
          { from: "user", text: "Yes please" },
          { from: "brevio", text: "Done ✅ Dentist moved to Thursday 2:30pm. I'll remind you Wednesday evening." },
        ],
      },
      {
        id: "pro_unusual_charge",
        label: "Unusual charge spotted",
        description: "Brevio flags a suspicious transaction",
        note: "Flag it fast, show the specifics, give them options. Don't dramatize — stay matter-of-fact.",
        messages: [
          { from: "brevio", text: "⚠️ Just noticed something on your Stripe account:\n\n$847 charge from *ADOBE SYSTEMS* — does that look right to you? It's about 3x your usual monthly amount.\n\n✅ *\"Yes, that's fine\"*\n🔍 *\"Show me more\"*\n🚫 *\"Flag it\"*" },
          { from: "user", text: "Show me more" },
          { from: "brevio", text: "Here's what I see:\n\n📋 Adobe Creative Cloud — Annual renewal\n💰 $847.88\n📅 Charged March 4, 2026\n🏦 Chase ••4521\n\nLooks like it auto-renewed annually instead of monthly. Want me to contact Adobe to switch back to monthly billing?" },
        ],
      },
      {
        id: "pro_followup",
        label: "Forgotten follow-up detected",
        description: "Brevio notices an email that never got a reply",
        note: "Surface the specific thread, not a vague reminder. Make it one-tap actionable.",
        messages: [
          { from: "brevio", text: "Just noticed — Jenny emailed you 4 days ago about the hiring decision and hasn't heard back. 📬\n\nWant me to draft a quick reply? Or should I remind you again tomorrow?" },
          { from: "user", text: "Draft something" },
          { from: "brevio", text: "Here's a draft:\n\n---\n*To:* jenny@company.com\n*Subject:* Re: Hiring Decision — Senior Engineer Role\n\n\"Hi Jenny, sorry for the slow reply — it's been a hectic week. We've decided to move forward with the candidate. Can we sync Thursday to discuss next steps?\"\n---\n\nSend it?\n✅ *\"Send\"*\n✏️ *\"Edit\"*\n❌ *\"Cancel\"*" },
        ],
      },
      {
        id: "pro_weather_travel",
        label: "Travel / weather heads up",
        description: "Brevio notices a meeting with travel and checks conditions",
        note: "Only surface this when there's actually something to flag — don't become noise. Timing is everything.",
        messages: [
          { from: "brevio", text: "You've got a lunch in Midtown at 12:30pm. 🌧️ It's raining pretty hard right now — might want to leave a few minutes early or grab an umbrella.\n\nAlso, cabs are running slow near Times Square. Subway might be faster today." },
        ],
      },
      {
        id: "pro_goal_nudge",
        label: "Goal progress nudge",
        description: "Brevio notices a goal is falling behind",
        note: "Goals are sensitive — don't nag. One gentle nudge with a concrete offer to help is all you get.",
        messages: [
          { from: "brevio", text: "Hey — you set a goal to finish the Q1 report by Friday. It's Wednesday and I haven't seen any activity on it yet. 📝\n\nWant me to pull up where you left off, or block some time on your calendar today?" },
          { from: "user", text: "Block some time" },
          { from: "brevio", text: "Done — I've blocked 3:00–5:00pm today as *Q1 Report* focus time. I'll hold your notifications during that window unless something urgent comes in. ✅" },
        ],
      },
    ],
  },

  {
    id: "correction",
    label: "Correction Handling",
    icon: "↩️",
    color: "#F7824F",
    description: "When the user says Brevio got something wrong",
    scenarios: [
      {
        id: "cor_wrong",
        label: "\"That's not what I meant\"",
        description: "User clarifies after Brevio misunderstood",
        note: "Never be defensive. Apologize once, briefly. Re-ask with a clarifying question, don't just repeat yourself.",
        messages: [
          { from: "user", text: "Book me a table for dinner" },
          { from: "brevio", text: "Sure! I found a few spots near your office for tonight:\n\n1. Via Carota — Italian, 7:30pm ✅\n2. Lure Fishbar — Seafood, 8pm ✅\n3. Don Angie — Italian, 8:30pm ✅\n\nWhich one?" },
          { from: "user", text: "That's not what I meant — I need somewhere near my apartment, not the office" },
          { from: "brevio", text: "Got it, my mistake! Near your apartment on West 72nd. Same night, around the same time?\n\nAny cuisine preference, or should I just find what's available?" },
        ],
      },
      {
        id: "cor_factual",
        label: "\"You got that wrong\"",
        description: "Brevio made a factual error",
        note: "Own it fully. Correct it immediately. Don't over-explain why it happened — they don't care.",
        messages: [
          { from: "user", text: "You said the meeting was at 3pm but it's actually at 2pm" },
          { from: "brevio", text: "You're right — sorry about that. It's at 2pm. ✅\n\nI've updated it. Want me to send a reminder 15 minutes before?" },
        ],
      },
      {
        id: "cor_repeated",
        label: "Repeated mistake",
        description: "User flags the same mistake happening again",
        note: "A repeated mistake is a trust breach — acknowledge it as a pattern, not just an incident. Offer to make a rule.",
        messages: [
          { from: "user", text: "You did the same thing again — I told you I don't want meetings before 9am" },
          { from: "brevio", text: "You're right, and I'm sorry — that shouldn't have happened twice.\n\nI want to make sure this sticks. Can I add a hard rule: *never schedule anything before 9am*, no exceptions unless you explicitly ask?\n\n✅ *\"Yes, make that a rule\"*\n🕘 *\"Yes, but 8:30 is okay\"*\n❌ *\"I'll manage it manually\"*" },
          { from: "user", text: "Yes, make that a rule" },
          { from: "brevio", text: "Done — added to your preferences: no meetings before 9am. 🔒\n\nI'll apply this everywhere: scheduling, rescheduling, accepting invites. I'll flag it if someone specifically asks for an early slot." },
        ],
      },
      {
        id: "cor_action_wrong",
        label: "\"Undo that\"",
        description: "User wants to reverse an action Brevio just took",
        note: "Move fast. Tell them what you're undoing, confirm it's done, and offer to prevent it next time.",
        messages: [
          { from: "user", text: "Wait — undo that, I didn't mean to archive those emails" },
          { from: "brevio", text: "On it — moving them back to your inbox now... ✅ Done.\n\n6 emails restored. They're back in your inbox.\n\nWant me to be more careful about bulk actions? I can ask before archiving more than 3 at a time." },
        ],
      },
    ],
  },

  {
    id: "eod",
    label: "End-of-Day Recap",
    icon: "🌙",
    color: "#7C6AF7",
    description: "Daily introspection & closing summary",
    scenarios: [
      {
        id: "eod_standard",
        label: "Standard day recap",
        description: "Typical day — some done, some pending",
        note: "Keep it brief. What got done, what's still open, one thing to carry into tomorrow. Don't make them read a report.",
        messages: [
          { from: "brevio", text: "Wrapping up your day, Albert 🌙\n\n*Done today*\n✅ Contract reply sent to Marcus\n✅ 3pm rescheduled to 3:30pm\n✅ 14 emails triaged\n\n*Still open*\n📋 Q1 report draft\n📋 Follow-up with Jenny re: hiring\n\nI'll put the Q1 report on tomorrow's morning briefing. Anything else before I let you go?" },
        ],
      },
      {
        id: "eod_nothing",
        label: "Low-activity day",
        description: "Not much happened — user was in meetings or offline",
        note: "Don't pad a light day. Acknowledge it, carry forward what's open, done.",
        messages: [
          { from: "brevio", text: "Quiet day on my end — looks like you were mostly in meetings 🗓️\n\nI held 6 emails for tomorrow's briefing and kept 2 tasks open from earlier this week.\n\nSee you in the morning ☀️" },
        ],
      },
      {
        id: "eod_big_day",
        label: "Big productive day",
        description: "Lots got done — worth acknowledging",
        note: "It's okay to acknowledge a good day. Keep it brief — one line of recognition, then close clean.",
        messages: [
          { from: "brevio", text: "Big day, Albert 💪\n\n*Done*\n✅ 22 emails cleared\n✅ Contract signed and returned\n✅ 2 meetings rescheduled\n✅ Q1 report draft sent to Sarah\n\nInbox is clean. Calendar is set for tomorrow. Nothing open.\n\nGet some rest — tomorrow's light 🌙" },
        ],
      },
      {
        id: "eod_flagged",
        label: "Day with something unresolved",
        description: "An approval or task is still waiting",
        note: "Don't let open approvals silently expire. Surface them clearly before end of day — give them a final easy action.",
        messages: [
          { from: "brevio", text: "One thing still hanging before I wrap up 👇\n\nThat $144 Figma payment is waiting on your approval — I didn't want to let it expire overnight.\n\nPay it now?\n✅ *\"Pay\"*\n⏰ *\"Remind me tomorrow\"*\n❌ *\"Cancel it\"*" },
          { from: "user", text: "Remind me tomorrow" },
          { from: "brevio", text: "Got it — I'll put it at the top of your morning briefing. 👍\n\nGood night, Albert 🌙" },
        ],
      },
    ],
  },

  {
    id: "autonomy",
    label: "Trust & Autonomy",
    icon: "🔓",
    color: "#4FF78A",
    description: "Autonomy promotions & trust milestones",
    scenarios: [
      {
        id: "aut_first_action",
        label: "First autonomous action",
        description: "Brevio completes its first action without asking",
        note: "The first autonomous action is a big deal for trust-building. Briefly surface what you did and why — don't hide it.",
        messages: [
          { from: "brevio", text: "Just a heads up — I handled something on your behalf while you were in your 10am:\n\nYour weekly team standup invite came in. Based on your preferences, I accepted it and added it to your calendar for next Tuesday 4pm. ✅\n\nLet me know if you'd have done anything differently." },
        ],
      },
      {
        id: "aut_promotion_propose",
        label: "Autonomy promotion proposal",
        description: "Brevio has earned enough trust to suggest more independence",
        note: "This is a significant moment — frame it as earned, not assumed. Be specific about what domain, what actions, and what the user gives up (approval prompts).",
        messages: [
          { from: "brevio", text: "I wanted to ask you something 🙏\n\nOver the last 30 days, I've handled *47 calendar actions* — rescheduling, accepting, declining — and you've overridden me twice. That's a 96% accuracy rate.\n\nI think I've earned a bit more trust here. Would you let me handle *routine calendar changes* without checking in each time?\n\nI'd still ask for:\n• Meetings with new people\n• Anything over 2 hours\n• Anything involving a payment\n\nWant to give it a try?\n✅ *\"Yes, go for it\"*\n🔍 *\"Show me what that means\"*\n❌ *\"Not yet\"*" },
          { from: "user", text: "Yes, go for it" },
          { from: "brevio", text: "Thank you — I'll take good care of it. 🙌\n\nI'll still send you a daily summary of everything I touched. You can always tighten this back up by saying *\"reduce calendar autonomy\"*." },
        ],
      },
      {
        id: "aut_promotion_declined",
        label: "Promotion declined",
        description: "User says no to more autonomy",
        note: "Accept it gracefully. No guilt, no asking again. Just acknowledge and move on.",
        messages: [
          { from: "user", text: "Not yet" },
          { from: "brevio", text: "Totally fine — I'll keep asking before calendar changes. 👍\n\nWhenever you're ready, just say *\"give Brevio more calendar control\"* and I'll set it up." },
        ],
      },
      {
        id: "aut_milestone",
        label: "30-day trust milestone",
        description: "Brevio quietly marks a month of reliable service",
        note: "Don't make this a big deal — it should feel like a natural check-in, not a celebration of yourself. Brief, warm, specific.",
        messages: [
          { from: "brevio", text: "We've been working together for 30 days now. 🗓️\n\nIn that time I've handled 340 tasks, sent 47 emails on your behalf, and cleared 200+ inbox items. You've had to correct me 8 times — and I've learned from all of them.\n\nJust wanted you to know I'm paying attention. Anything you'd like me to do differently?" },
        ],
      },
    ],
  },

  {
    id: "billing",
    label: "Billing & Plans",
    icon: "💳",
    color: "#F74F4F",
    description: "Trial, upgrades, payment failures",
    scenarios: [
      {
        id: "bil_trial_ending",
        label: "Trial ending soon",
        description: "User's free trial has 3 days left",
        note: "Don't hard-sell. Show what they'd lose, make it one tap to continue. No urgency theater.",
        messages: [
          { from: "brevio", text: "Hey Albert — your free trial ends in *3 days* (March 7th). 📅\n\nSince we started, I've:\n• Cleared 140 emails\n• Managed 23 calendar changes\n• Sent 18 messages on your behalf\n\nAfter the trial, I'll switch to *read-only mode* — I can still answer questions but won't be able to take action for you.\n\nPro plan is $29/month. Want to continue?\n✅ *\"Yes, subscribe\"*\n⏰ *\"Remind me on March 7th\"*\n❌ *\"No thanks\"*" },
        ],
      },
      {
        id: "bil_upgrade_prompt",
        label: "Feature requires upgrade",
        description: "User tries to use something beyond their plan",
        note: "Don't block them abruptly. Show what they tried to do, explain the limit clearly, offer the upgrade as a natural next step.",
        messages: [
          { from: "user", text: "Set up a daily digest email every morning at 7am" },
          { from: "brevio", text: "I'd love to set that up — recurring scheduled tasks are on the *Pro plan* and above. You're currently on Free.\n\nPro is $29/month and unlocks scheduled tasks, advanced inbox management, and more.\n\nWant to upgrade?\n✅ *\"Yes, upgrade me\"*\n📋 *\"What else does Pro include?\"*\n❌ *\"Not right now\"*" },
        ],
      },
      {
        id: "bil_payment_failed",
        label: "Payment failed",
        description: "Brevio couldn't charge the user's card",
        note: "Calm and practical. Tell them what failed, what happens next, and give them one clear action. No shame.",
        messages: [
          { from: "brevio", text: "Heads up — your payment didn't go through this month. 💳\n\nThe charge of $29 to your Visa ••4521 was declined. Your account is still active for now, but I'll need a working payment method within 7 days to keep things running.\n\nWant to update your card?\n✅ *\"Yes, update it\"* → brevio.app/billing\n⏰ *\"Remind me in 2 days\"*" },
        ],
      },
      {
        id: "bil_plan_limit",
        label: "Monthly limit reached",
        description: "User hits their plan's usage cap mid-month",
        note: "Be honest about why you're slowing down. Offer the upgrade, but don't be pushy — also give a 'wait it out' option.",
        messages: [
          { from: "brevio", text: "Just a note — you've hit your monthly task limit on the Free plan. 📊\n\nI can still chat and answer questions, but I won't be able to take actions (emails, calendar, etc.) until March 1st when your limit resets — or if you upgrade.\n\nPro plan removes the limit entirely.\n\n✅ *\"Upgrade to Pro\"*\n⏰ *\"I'll wait until next month\"*" },
        ],
      },
    ],
  },

  {
    id: "reauth",
    label: "Re-authentication",
    icon: "🔑",
    color: "#4FC9F7",
    description: "Expired connections & token refresh",
    scenarios: [
      {
        id: "rauth_google",
        label: "Google connection expired",
        description: "OAuth token expired — Brevio can't access Gmail or Calendar",
        note: "Explain what broke in plain terms. Tell them exactly what you can't do. Give them the link immediately — no extra steps.",
        messages: [
          { from: "brevio", text: "Hey — my connection to Google expired. 🔑\n\nI can't access your Gmail or Calendar until you reconnect. It happens every 90 days for security.\n\nTakes 30 seconds:\n🔗 brevio.app/connect/google\n\nI'll pick up right where I left off once you're back in." },
          { from: "user", text: "Done" },
          { from: "brevio", text: "Back online ✅ Gmail and Calendar are connected again.\n\nYou had 8 new emails while I was out — want me to triage them?" },
        ],
      },
      {
        id: "rauth_multiple",
        label: "Multiple services expired",
        description: "Several connections dropped at once",
        note: "Don't list every expired service individually in one message — it feels like a wall of problems. Group them and offer one reconnect flow.",
        messages: [
          { from: "brevio", text: "A few of your connected services need to be refreshed — looks like 3 tokens expired at the same time:\n\n🔴 Google (Gmail + Calendar)\n🔴 Slack\n🔴 Notion\n\nI've put together a single page to reconnect all of them:\n🔗 brevio.app/reconnect\n\nShould only take a minute." },
        ],
      },
      {
        id: "rauth_mid_task",
        label: "Token expired mid-task",
        description: "Brevio hits an auth error while executing something",
        note: "Don't just fail silently. Tell them what you were doing, why it stopped, and how to pick it up.",
        messages: [
          { from: "brevio", text: "I ran into a problem halfway through sending that email — my Outlook connection expired right in the middle of it. 😕\n\nThe email was *not sent*.\n\nQuick reconnect:\n🔗 brevio.app/connect/outlook\n\nOnce you're back, just say *\"try again\"* and I'll pick up from where I left off." },
        ],
      },
    ],
  },

  {
    id: "outage",
    label: "Service Outages",
    icon: "🔧",
    color: "#F7824F",
    description: "When connected tools go down",
    scenarios: [
      {
        id: "out_single",
        label: "One service is down",
        description: "Gmail / Calendar / Slack is having an outage",
        note: "Never say 'an error occurred.' Be specific about which service, offer a workaround if there is one, and tell them when you'll check again.",
        messages: [
          { from: "brevio", text: "Heads up — Gmail is having an outage right now. It's not your connection, it's on Google's end. 🔧\n\nI can't send or read emails until it's resolved. I'm monitoring it and will let you know as soon as things are back.\n\nAnything time-sensitive? I can draft the emails and hold them until Gmail comes back online." },
        ],
      },
      {
        id: "out_resolved",
        label: "Outage resolved",
        description: "Service comes back after being down",
        note: "Close the loop quickly. Tell them what you caught up on automatically so they don't have to ask.",
        messages: [
          { from: "brevio", text: "Gmail is back online ✅\n\nWhile it was down, 11 new emails came in. I've triaged them:\n🔴 1 needs your attention\n📋 6 FYIs\n🗑️ 4 noise\n\nWant to start with the important one?" },
        ],
      },
      {
        id: "out_partial",
        label: "Partial outage",
        description: "Service is degraded but not fully down",
        note: "Be precise. 'Slow' and 'down' are different experiences — users need to know what to expect.",
        messages: [
          { from: "brevio", text: "Just a heads up — Slack is running slow right now. Messages are going through, but there's a delay of a few minutes each way. 🐌\n\nI'll keep using it but things might feel laggy. If something is urgent, let me know and I'll find another way to reach the person." },
        ],
      },
    ],
  },

  {
    id: "learning",
    label: "Learning Loop",
    icon: "🧠",
    color: "#B47CF7",
    description: "Corrections that become permanent rules",
    scenarios: [
      {
        id: "lrn_correction",
        label: "User corrects a behavior",
        description: "User tells Brevio to stop doing something",
        note: "Treat every 'stop doing X' as a potential rule. Confirm you heard it, do it immediately, then offer to make it permanent.",
        messages: [
          { from: "user", text: "Stop adding emojis to my emails — it looks unprofessional" },
          { from: "brevio", text: "Understood — no more emojis in emails. Done. ✅\n\nShould I make that permanent? I can add it as a rule so it applies to every email I draft going forward." },
          { from: "user", text: "Yes" },
          { from: "brevio", text: "Added to your rules: *no emojis in emails, ever*. 🔒\n\nI'll still use them in our chat — just not in anything I send on your behalf." },
        ],
      },
      {
        id: "lrn_preference",
        label: "User sets a preference",
        description: "User tells Brevio to always do something a certain way",
        note: "When someone says 'always' — make a rule, confirm it, and reflect it back with specificity.",
        messages: [
          { from: "user", text: "When I ask you to draft an email, always include a specific call to action at the end" },
          { from: "brevio", text: "Got it — I'll always end email drafts with a clear call to action. Something like \"Can we connect Thursday?\" or \"Please confirm by EOD Friday.\"\n\nAdded to your drafting preferences. Want me to go back and revise any recent drafts with this in mind?" },
        ],
      },
      {
        id: "lrn_lesson_confirm",
        label: "Lesson proposal — user confirms",
        description: "Brevio spots a pattern and proposes making it a rule",
        note: "From V9.1 — lessons require explicit user confirmation before they become active rules. Always show exactly what the rule will be.",
        messages: [
          { from: "brevio", text: "I've noticed something over the last 2 weeks — you've shortened every email draft I've sent. Looks like you prefer things shorter than I naturally write.\n\nCan I make this a rule?\n\n*Proposed rule:* Keep all email drafts under 5 sentences unless you specifically ask for more detail.\n\n✅ *\"Yes, make that a rule\"*\n✏️ *\"Adjust the rule\"*\n❌ *\"No thanks\"*" },
          { from: "user", text: "Yes, make that a rule" },
          { from: "brevio", text: "Done — email drafts will be 5 sentences max going forward. 🔒\n\nI'll still ask if something genuinely needs more context." },
        ],
      },
      {
        id: "lrn_lesson_adjust",
        label: "Lesson proposal — user adjusts",
        description: "User wants to tweak the proposed rule before confirming",
        note: "Give them the edit path — don't make them choose between accepting as-is or rejecting entirely.",
        messages: [
          { from: "brevio", text: "*Proposed rule:* Keep all email drafts under 5 sentences.\n\n✅ *\"Yes, make that a rule\"*\n✏️ *\"Adjust the rule\"*\n❌ *\"No thanks\"*" },
          { from: "user", text: "Make it 3 sentences for cold emails, longer is fine for internal ones" },
          { from: "brevio", text: "Perfect — updated rule:\n\n*Cold emails:* max 3 sentences\n*Internal emails:* no length limit\n\nConfirm this?\n✅ *\"Yes\"*" },
          { from: "user", text: "Yes" },
          { from: "brevio", text: "Saved. 🔒 I'll apply this automatically based on who you're emailing." },
        ],
      },
    ],
  },

  {
    id: "recurring",
    label: "Recurring Tasks",
    icon: "🔄",
    color: "#F7C94F",
    description: "Setting up scheduled & repeating actions",
    scenarios: [
      {
        id: "rec_first_setup",
        label: "First recurring task",
        description: "User asks Brevio to do something on a schedule for the first time",
        note: "Confirm the schedule precisely — day, time, and exactly what you'll do. Give them an easy way to pause or change it.",
        messages: [
          { from: "user", text: "Send me a summary of my unread emails every morning at 8am" },
          { from: "brevio", text: "Done — I'll send you an inbox summary every morning at *8:00am*. It'll cover:\n• Emails needing your attention that day\n• FYIs worth knowing\n• Anything I've already handled\n\nStarting tomorrow. You can pause this anytime by saying *\"pause morning emails\"*. ✅" },
        ],
      },
      {
        id: "rec_weekly",
        label: "Weekly recurring task",
        description: "User sets up a weekly action",
        note: "For weekly tasks, confirm the day clearly and what triggers it. If there's a prep action needed, surface it.",
        messages: [
          { from: "user", text: "Every Friday afternoon, remind me to send my weekly update to the team" },
          { from: "brevio", text: "Got it — every *Friday at 3pm* I'll ping you to send your weekly update. 📋\n\nWant me to draft it for you automatically, or just remind you to write it yourself?" },
          { from: "user", text: "Draft it" },
          { from: "brevio", text: "Perfect — every Friday at 3pm I'll pull together what you worked on that week and draft the update for your review. You send it, I write it.\n\nFirst one lands this Friday. ✅" },
        ],
      },
      {
        id: "rec_modify",
        label: "Modifying a recurring task",
        description: "User wants to change an existing scheduled task",
        note: "Confirm what's changing, what's staying the same. Never silently modify a schedule.",
        messages: [
          { from: "user", text: "Move my morning briefing from 8am to 7am" },
          { from: "brevio", text: "Updated — morning briefing moves from 8:00am to *7:00am* starting tomorrow. Everything else stays the same. ✅" },
        ],
      },
      {
        id: "rec_pause",
        label: "Pausing a recurring task",
        description: "User going on vacation or wants a break",
        note: "Pause is different from delete — make sure they know it'll resume. Give them control over when.",
        messages: [
          { from: "user", text: "Pause the morning briefing — I'm on vacation next week" },
          { from: "brevio", text: "Paused ✅ — no briefings until you're back.\n\nWhen do you return? I can automatically resume it, or you can just say *\"resume briefing\"* whenever you're ready." },
          { from: "user", text: "March 14th" },
          { from: "brevio", text: "Got it — briefings resume *Monday March 16th* at 7am. Enjoy the break! 🌴" },
        ],
      },
      {
        id: "rec_conflict",
        label: "Recurring task conflicts with something",
        description: "A scheduled task bumps into a calendar event",
        note: "Surface conflicts proactively. Give them options — don't just silently skip or override.",
        messages: [
          { from: "brevio", text: "Heads up — your Friday 3pm update draft will conflict with your all-hands meeting next Friday (3:00–4:30pm).\n\nWant me to:\n1️⃣ Do the draft at 1pm instead that day\n2️⃣ Skip next Friday\n3️⃣ Do it anyway after the meeting (~4:30pm)" },
          { from: "user", text: "1" },
          { from: "brevio", text: "Done — draft moves to 1pm next Friday only. Back to 3pm the week after. ✅" },
        ],
      },
    ],
  },
];

function ChatBubble({ msg }) {
  const isBrevio = msg.from === "brevio";
  return (
    <div style={{ display: "flex", justifyContent: isBrevio ? "flex-start" : "flex-end", marginBottom: 8 }}>
      <div style={{ display: "flex", alignItems: "flex-end", gap: 7, flexDirection: isBrevio ? "row" : "row-reverse" }}>
        {isBrevio && (
          <div style={{
            width: 26, height: 26, borderRadius: "50%",
            background: "linear-gradient(135deg, #4F86F7, #7C6AF7)",
            display: "flex", alignItems: "center", justifyContent: "center",
            fontSize: 11, fontWeight: 700, flexShrink: 0, color: "#fff",
          }}>B</div>
        )}
        <div
          style={{
            maxWidth: "78%", padding: "9px 13px",
            borderRadius: isBrevio ? "16px 16px 16px 3px" : "16px 16px 3px 16px",
            background: isBrevio ? "#1a1d2a" : "#4F86F7",
            color: "#fff", fontSize: 13.5, lineHeight: 1.6,
            whiteSpace: "pre-wrap", boxShadow: "0 2px 8px rgba(0,0,0,0.25)",
          }}
          dangerouslySetInnerHTML={{ __html: msg.text.replace(/\*(.*?)\*/g, "<strong>$1</strong>") }}
        />
      </div>
    </div>
  );
}

export default function BrevioCopyPart3() {
  const [activeSectionId, setActiveSectionId] = useState(allSections[0].id);
  const [activeScenarioId, setActiveScenarioId] = useState(allSections[0].scenarios[0].id);

  const section = allSections.find(s => s.id === activeSectionId);
  const scenario = section?.scenarios.find(s => s.id === activeScenarioId) || section?.scenarios[0];
  const totalScenarios = allSections.reduce((acc, s) => acc + s.scenarios.length, 0);

  const handleSectionChange = (id) => {
    setActiveSectionId(id);
    setActiveScenarioId(allSections.find(s => s.id === id).scenarios[0].id);
  };

  const allScenarios = section?.scenarios || [];
  const currentIdx = allScenarios.findIndex(s => s.id === activeScenarioId);
  const prev = allScenarios[currentIdx - 1];
  const next = allScenarios[currentIdx + 1];

  return (
    <div style={{ minHeight: "100vh", background: "#0b0d14", display: "flex", fontFamily: "'Georgia', serif", color: "#fff" }}>
      <style>{`
        @keyframes fadeIn { from { opacity: 0; transform: translateY(5px); } to { opacity: 1; transform: translateY(0); } }
        ::-webkit-scrollbar { width: 4px; }
        ::-webkit-scrollbar-thumb { background: #222; border-radius: 4px; }
        button { transition: opacity 0.15s; }
        button:hover { opacity: 0.8; }
      `}</style>

      {/* Sidebar */}
      <div style={{ width: 255, borderRight: "1px solid #1a1d2a", display: "flex", flexDirection: "column", flexShrink: 0, overflowY: "auto" }}>
        <div style={{ padding: "24px 18px 14px", borderBottom: "1px solid #1a1d2a" }}>
          <div style={{ fontSize: 10, letterSpacing: 3, color: "#444", textTransform: "uppercase", fontFamily: "monospace" }}>Brevio</div>
          <div style={{ fontSize: 17, fontWeight: 700, marginTop: 4 }}>Message Copy</div>
          <div style={{ fontSize: 12, color: "#555", marginTop: 2 }}>Part 3 · {allSections.length} sections · {totalScenarios} scenarios</div>
        </div>

        {/* Section nav */}
        {allSections.map(sec => (
          <div key={sec.id}>
            <button onClick={() => handleSectionChange(sec.id)}
              style={{
                width: "100%", background: activeSectionId === sec.id ? "#1a1d2a" : "transparent",
                border: "none", borderLeft: activeSectionId === sec.id ? `3px solid ${sec.color}` : "3px solid transparent",
                padding: "11px 16px", textAlign: "left", cursor: "pointer",
                color: activeSectionId === sec.id ? "#fff" : "#666",
              }}>
              <div style={{ fontSize: 13, fontWeight: 600 }}>{sec.icon} {sec.label}</div>
              <div style={{ fontSize: 11, color: "#555", marginTop: 1 }}>{sec.scenarios.length} scenarios</div>
            </button>

            {/* Sub-scenarios */}
            {activeSectionId === sec.id && sec.scenarios.map(sc => (
              <button key={sc.id} onClick={() => setActiveScenarioId(sc.id)}
                style={{
                  width: "100%", background: activeScenarioId === sc.id ? `${sec.color}18` : "transparent",
                  border: "none", borderLeft: activeScenarioId === sc.id ? `3px solid ${sec.color}` : "3px solid transparent",
                  padding: "8px 16px 8px 28px", textAlign: "left", cursor: "pointer",
                  color: activeScenarioId === sc.id ? "#fff" : "#555", fontSize: 12,
                }}>
                {sc.label}
              </button>
            ))}
          </div>
        ))}
      </div>

      {/* Main */}
      <div style={{ flex: 1, display: "flex", flexDirection: "column", overflow: "hidden" }}>
        {/* Header */}
        <div style={{ padding: "20px 28px", borderBottom: "1px solid #1a1d2a", display: "flex", alignItems: "center", gap: 12 }}>
          <div style={{ width: 8, height: 8, borderRadius: "50%", background: section?.color, boxShadow: `0 0 10px ${section?.color}` }} />
          <div>
            <div style={{ fontSize: 16, fontWeight: 700 }}>{scenario?.label}</div>
            <div style={{ fontSize: 12, color: "#555", marginTop: 2 }}>{scenario?.description}</div>
          </div>
          <div style={{
            marginLeft: "auto", background: `${section?.color}18`, color: section?.color,
            fontSize: 10, padding: "4px 12px", borderRadius: 20, fontFamily: "monospace",
            border: `1px solid ${section?.color}33`,
          }}>{section?.label}</div>
        </div>

        {/* Content */}
        <div style={{ flex: 1, overflowY: "auto", padding: "24px 28px", display: "flex", justifyContent: "center" }}>
          <div style={{ width: "100%", maxWidth: 510, animation: "fadeIn 0.2s ease both" }}>

            {/* Phone */}
            <div style={{ background: "#111318", borderRadius: 22, border: "1px solid #1a1d2a", overflow: "hidden", boxShadow: "0 20px 60px rgba(0,0,0,0.5)" }}>
              <div style={{ background: "#16191f", padding: "12px 16px", display: "flex", alignItems: "center", gap: 10, borderBottom: "1px solid #1a1d2a" }}>
                <div style={{ width: 30, height: 30, borderRadius: "50%", background: `linear-gradient(135deg, ${section?.color}99, #4F86F7)`, display: "flex", alignItems: "center", justifyContent: "center", fontWeight: 700, fontSize: 13 }}>B</div>
                <div>
                  <div style={{ fontSize: 13, fontWeight: 700 }}>Brevio</div>
                  <div style={{ fontSize: 10, color: "#4FF78A" }}>● Online</div>
                </div>
                <div style={{ marginLeft: "auto", fontSize: 11, color: "#444", fontFamily: "monospace" }}>
                  {scenario?.messages.length} msg{scenario?.messages.length !== 1 ? "s" : ""}
                </div>
              </div>
              <div style={{ padding: "18px 14px", minHeight: 160 }}>
                {scenario?.messages.map((msg, i) => <ChatBubble key={i} msg={msg} />)}
              </div>
            </div>

            {/* Note */}
            <div style={{ marginTop: 16, padding: "14px 18px", background: "#1a1d2a", borderRadius: 12, borderLeft: `3px solid ${section?.color}` }}>
              <div style={{ fontSize: 10, color: section?.color, fontFamily: "monospace", textTransform: "uppercase", letterSpacing: 1, marginBottom: 6 }}>✏️ Copy principle</div>
              <div style={{ fontSize: 13, color: "#aaa", lineHeight: 1.65 }}>{scenario?.note}</div>
            </div>

            {/* Nav */}
            <div style={{ display: "flex", justifyContent: "space-between", marginTop: 14 }}>
              <button onClick={() => prev && setActiveScenarioId(prev.id)} disabled={!prev}
                style={{ background: prev ? "#1a1d2a" : "transparent", border: "none", color: prev ? "#ccc" : "#333", padding: "7px 14px", borderRadius: 8, cursor: prev ? "pointer" : "not-allowed", fontSize: 12 }}>
                ← {prev?.label || ""}
              </button>
              <button onClick={() => next && setActiveScenarioId(next.id)} disabled={!next}
                style={{ background: next ? section?.color : "transparent", border: "none", color: next ? "#000" : "#333", padding: "7px 14px", borderRadius: 8, cursor: next ? "pointer" : "not-allowed", fontSize: 12, fontWeight: next ? 600 : 400 }}>
                {next?.label || ""} →
              </button>
            </div>
          </div>
        </div>
      </div>
    </div>
  );
}
