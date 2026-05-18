import { useState } from "react";

const sections = {
  briefing: {
    label: "Morning Briefing",
    color: "#F7C94F",
    icon: "☀️",
    description: "Sent daily at user's preferred time",
    scenarios: [
      {
        id: "briefing_standard",
        label: "Standard day",
        description: "Typical morning with a mix of items",
        messages: [
          {
            from: "brevio",
            text: "Good morning, Albert ☀️\n\nHere's your day at a glance — *Tuesday, March 5th*.",
          },
          {
            from: "brevio",
            text: "📅 *Calendar*\n3 meetings today\n\n• 9:00am — Investor call (Marcus Chen) *[45 min]*\n• 12:30pm — Lunch with Sarah *[1 hr]*\n• 4:00pm — Team standup *[30 min]*\n\n⚠️ Heads up: your 9am and 12:30pm are tight — you'll have about 10 minutes between the call ending and leaving for lunch.",
          },
          {
            from: "brevio",
            text: "📬 *Inbox*\n14 new emails since yesterday\n\n🔴 *2 need you today*\n• Marcus — contract sign-off by Friday *(reply drafted, say \"send\" to approve)*\n• Stripe — unusual charge on account *(flagged, say \"review\" to see it)*\n\n📋 *5 FYIs* — I'll hold these unless you ask\n🗑️ *7 noise* — archived",
          },
          {
            from: "brevio",
            text: "✅ *Tasks*\n2 open from yesterday\n• Finish Q1 report draft\n• Follow up with Jenny re: hiring\n\n*What would you like to tackle first today?*",
          },
        ],
        note: "Lead with calendar — it's time-sensitive. Then inbox, then tasks. Always surface conflicts proactively. End with an open invitation, not a full stop.",
      },
      {
        id: "briefing_light",
        label: "Light day",
        description: "Nothing urgent, clear schedule",
        messages: [
          {
            from: "brevio",
            text: "Good morning, Albert ☀️\n\nYou've got a clear one today — *no meetings*, nothing urgent in your inbox.\n\nPerfect day to make progress on something that matters. You mentioned wanting to get ahead on the Q1 report — want me to pull up where you left off?",
          },
        ],
        note: "When there's nothing urgent, don't pad it out. A short, energizing message is better than a long one with nothing to say. Use it to nudge toward a goal.",
      },
      {
        id: "briefing_heavy",
        label: "Hectic day",
        description: "Back-to-back schedule, lots of urgent items",
        messages: [
          {
            from: "brevio",
            text: "Morning, Albert — heads up, today's a full one. ☕\n\n📅 *6 meetings* — back to back from 9am to 5pm\n📬 *3 emails need action* before your first call\n⚠️ *1 conflict* — your 2pm and 3pm overlap by 15 minutes\n\nI'd suggest handling the 3 emails now (takes ~5 min) and I can reschedule the 3pm conflict. Want me to do that?",
          },
          { from: "user", text: "Yes please" },
          {
            from: "brevio",
            text: "Done — moved your 3pm to 3:30pm, confirmed with the other attendees. ✅\n\nHere are the 3 emails to handle before 9am 👇",
          },
        ],
        note: "Heavy days need triage, not a list. Lead with the conflict, propose a fix, get buy-in, execute. Don't overwhelm — sequence it.",
      },
      {
        id: "briefing_missed",
        label: "User missed yesterday",
        description: "User was unresponsive all of yesterday",
        messages: [
          {
            from: "brevio",
            text: "Good morning, Albert ☀️\n\nLooks like yesterday got away from you — no worries. Here's what carried over:\n\n⏰ *2 things from yesterday still need attention*\n• Contract reply to Marcus *(was due yesterday — I held off sending)*\n• Invoice approval for $2,400 *(pending your sign-off)*\n\nWant to handle those first before today's rundown?",
          },
        ],
        note: "Never make them feel bad about going dark. Lead with what's still outstanding and what you *didn't* do without their approval. Builds trust.",
      },
    ],
  },

  approval: {
    label: "Approval Requests",
    color: "#4F86F7",
    icon: "🔐",
    description: "When Brevio needs sign-off before acting",
    scenarios: [
      {
        id: "approval_email",
        label: "Sending an email",
        description: "Brevio drafted a reply and wants to send it",
        messages: [
          {
            from: "brevio",
            text: "Before I send this, here's exactly what will go out:\n\n---\n*To:* marcus@venturefirm.com\n*Subject:* Re: Contract Review\n\n\"Hi Marcus, confirming I'll have everything reviewed and back to you by Friday EOD. Thanks for your patience.\"\n---\n\nSend it?\n\n✅ *\"Send\"*\n✏️ *\"Edit\"*\n❌ *\"Cancel\"*",
          },
          { from: "user", text: "Send" },
          { from: "brevio", text: "Sent! ✅ I'll let you know if Marcus replies." },
        ],
        note: "Show the full email, not a summary. Subject line, recipient, body — all of it. One-word approvals ('Send') should always work.",
      },
      {
        id: "approval_calendar",
        label: "Modifying a calendar event",
        description: "Brevio wants to reschedule or accept a meeting",
        messages: [
          {
            from: "brevio",
            text: "I'd like to reschedule your 3pm with the team:\n\n📅 *Moving:* Team standup\n⏰ *From:* Tuesday 3:00pm\n⏰ *To:* Tuesday 3:30pm\n👥 *Notifying:* 4 attendees\n\nOkay to reschedule?\n\n✅ *\"Yes\"*\n🕐 *\"Different time\"*\n❌ *\"No\"*",
          },
          { from: "user", text: "Yes" },
          { from: "brevio", text: "Done — moved to 3:30pm, all 4 attendees notified. ✅" },
        ],
        note: "Calendar changes affect other people — always show who's being notified. Keep the confirmation specific (4 attendees, not 'your team').",
      },
      {
        id: "approval_payment",
        label: "Making a payment",
        description: "Brevio wants to pay an invoice or make a purchase",
        messages: [
          {
            from: "brevio",
            text: "I found the invoice. Here's what I'd pay:\n\n💳 *To:* Figma Inc.\n💰 *Amount:* $144.00\n📋 *For:* Annual subscription renewal\n🏦 *From:* Chase ••4521\n\nThis is above your $100 auto-approve limit, so I need your go-ahead.\n\nPay it?\n\n✅ *\"Pay\"*\n👀 *\"Show me the invoice\"*\n❌ *\"Don't pay\"*",
          },
          { from: "user", text: "Show me the invoice" },
          {
            from: "brevio",
            text: "Here's the invoice:\n📎 figma-invoice-2026.pdf\n\nEverything look good?\n\n✅ *\"Pay\"*\n❌ *\"Cancel\"*",
          },
          { from: "user", text: "Pay" },
          { from: "brevio", text: "Paid ✅ Receipt saved. You're covered through March 2027." },
        ],
        note: "Payments need more context than anything else. Show the limit threshold so users understand *why* you're asking. Always offer 'show me the invoice' before committing.",
      },
      {
        id: "approval_sensitive",
        label: "Sensitive or irreversible action",
        description: "Deleting something, cancelling a subscription, etc.",
        messages: [
          {
            from: "brevio",
            text: "Just to confirm — you want me to *cancel your Notion subscription*?\n\n⚠️ This will end your access at the end of the billing period (March 31st) and *can't be undone through me*.\n\nYou'd need to re-subscribe manually if you change your mind.\n\nStill want to cancel?\n\n✅ *\"Yes, cancel it\"*\n❌ *\"Actually, keep it\"*",
          },
          { from: "user", text: "Yes, cancel it" },
          {
            from: "brevio",
            text: "Cancelled. Your access runs through March 31st. ✅\n\nI've made a note in case you want to revisit this later.",
          },
        ],
        note: "Irreversible actions need a clear warning — but not a lecture. One sentence on the consequence, one on what happens next. Never make them feel stupid for asking.",
      },
      {
        id: "approval_timeout",
        label: "Approval request timed out",
        description: "User didn't respond to a pending approval",
        messages: [
          {
            from: "brevio",
            text: "Hey — I'm still holding that email to Marcus pending your approval. Just want to make sure it doesn't get forgotten.\n\nStill want me to send it?\n\n✅ *\"Send\"*\n❌ *\"Cancel\"*\n\n*(I'll drop it if I don't hear back by end of day)*",
          },
        ],
        note: "One reminder, then let it go. Tell them exactly when you'll drop it so they don't feel anxious about a dangling task.",
      },
    ],
  },

  skip: {
    label: "Skip Paths",
    color: "#4FF78A",
    icon: "⏭️",
    description: "When users don't want to answer questions",
    scenarios: [
      {
        id: "skip_single",
        label: "Skipping one question",
        description: "User skips a single onboarding question",
        messages: [
          {
            from: "brevio",
            text: "*Question 3 of 5*\nHow comfortable are you with me taking action without checking in?\n\n1️⃣ Always ask me first\n2️⃣ Ask for big things, just do small stuff\n3️⃣ Use your judgment — I trust you",
          },
          { from: "user", text: "skip" },
          {
            from: "brevio",
            text: "No problem — I'll default to asking you before anything important. You can always change this later by saying *\"update my preferences\"*.\n\n*Question 4 of 5* 👇",
          },
        ],
        note: "Acknowledge the skip, set a safe default, and tell them how to change it later. Then move on immediately — don't dwell.",
      },
      {
        id: "skip_all",
        label: "Skipping all questions",
        description: "User skips the entire discovery flow",
        messages: [
          {
            from: "brevio",
            text: "*Question 1 of 5*\nWhat best describes you?\n\n1️⃣ I run a business or team\n2️⃣ I'm a busy professional\n3️⃣ I'm managing a packed personal life\n4️⃣ A mix of all of the above",
          },
          { from: "user", text: "skip all" },
          {
            from: "brevio",
            text: "Got it — we can skip setup entirely. 👍\n\nI'll start with sensible defaults and learn what you like as we go. That works too.\n\nWhat can I help you with right now?",
          },
        ],
        note: "If they want to skip everything, let them. Don't negotiate. Pivot immediately to 'what do you need?' — that's the fastest path to value anyway.",
      },
      {
        id: "skip_oauth",
        label: "Skipping OAuth connection",
        description: "User doesn't want to connect any services",
        messages: [
          {
            from: "brevio",
            text: "Which of these do you use?\n\n1️⃣ Gmail\n2️⃣ Google Calendar\n3️⃣ Outlook / Microsoft 365\n4️⃣ Slack\n5️⃣ Something else\n\nOr say *\"skip\"* — you can always connect later.",
          },
          { from: "user", text: "skip" },
          {
            from: "brevio",
            text: "Sure! No pressure.\n\nWhenever you're ready, just say *\"connect my Gmail\"* (or any service) and I'll walk you through it.\n\nIn the meantime, I can still help with lots of things that don't need a connection — research, drafting, reminders, and more.\n\nWhat would you like to try?",
          },
        ],
        note: "Don't make them feel like they're getting a lesser experience. Immediately tell them what they CAN do without connecting anything.",
      },
      {
        id: "skip_edge_agent",
        label: "Skipping edge agent install",
        description: "User doesn't want to install the desktop agent",
        messages: [
          {
            from: "brevio",
            text: "One optional step — if you want me to work with files and apps on your Mac directly, you can install my desktop agent:\n\n⬇️ brevio.app/install/mac\n\nTakes about 2 minutes. Or say *\"skip\"* — totally optional.",
          },
          { from: "user", text: "skip" },
          {
            from: "brevio",
            text: "No problem — you don't need it for most things.\n\nIf you ever want local file access later, just say *\"install desktop agent\"* and I'll send you the link.",
          },
        ],
        note: "Edge agent install should always feel optional. Two sentences max when they skip — acknowledge, leave the door open, done.",
      },
      {
        id: "skip_mid_task",
        label: "Skipping mid-task clarification",
        description: "User doesn't want to answer a clarifying question",
        messages: [
          {
            from: "brevio",
            text: "Quick question before I send this — should I CC anyone on the reply to Marcus?",
          },
          { from: "user", text: "just send it" },
          {
            from: "brevio",
            text: "On it. ✅\n\nSent — no CC.",
          },
        ],
        note: "When a user says 'just do it', do it. Don't ask again, don't confirm the confirmation. Trust is built by acting decisively when given permission.",
      },
      {
        id: "skip_preference_update",
        label: "Skipping a preference update prompt",
        description: "Brevio noticed a pattern and suggests updating preferences — user declines",
        messages: [
          {
            from: "brevio",
            text: "I've noticed you usually prefer replies without CC. Want me to make that my default going forward?",
          },
          { from: "user", text: "no thanks" },
          {
            from: "brevio",
            text: "Got it — I'll keep asking each time. 👍",
          },
        ],
        note: "When they decline a suggested preference update, confirm what the new behavior IS (keep asking), not what it ISN'T. Clarity prevents confusion later.",
      },
    ],
  },
};

function ChatBubble({ msg }) {
  const isBrevio = msg.from === "brevio";
  return (
    <div style={{ display: "flex", justifyContent: isBrevio ? "flex-start" : "flex-end", marginBottom: 8 }}>
      <div style={{ display: "flex", alignItems: "flex-end", gap: 7, flexDirection: isBrevio ? "row" : "row-reverse" }}>
        {isBrevio && (
          <div style={{
            width: 25, height: 25, borderRadius: "50%",
            background: "linear-gradient(135deg, #4F86F7, #7C6AF7)",
            display: "flex", alignItems: "center", justifyContent: "center",
            fontSize: 11, fontWeight: 700, flexShrink: 0,
          }}>B</div>
        )}
        <div style={{
          maxWidth: "78%", padding: "9px 13px",
          borderRadius: isBrevio ? "16px 16px 16px 3px" : "16px 16px 3px 16px",
          background: isBrevio ? "#1a1d2a" : "#4F86F7",
          color: "#fff", fontSize: 13.5, lineHeight: 1.6,
          whiteSpace: "pre-wrap", boxShadow: "0 2px 6px rgba(0,0,0,0.3)",
        }}
          dangerouslySetInnerHTML={{ __html: msg.text.replace(/\*(.*?)\*/g, "<strong>$1</strong>") }}
        />
      </div>
    </div>
  );
}

export default function BrevioCopyPart2() {
  const [activeSection, setActiveSection] = useState("briefing");
  const [activeScenario, setActiveScenario] = useState(sections.briefing.scenarios[0].id);

  const sec = sections[activeSection];
  const scenario = sec.scenarios.find(s => s.id === activeScenario) || sec.scenarios[0];

  const handleSectionChange = (key) => {
    setActiveSection(key);
    setActiveScenario(sections[key].scenarios[0].id);
  };

  return (
    <div style={{ minHeight: "100vh", background: "#0b0d14", display: "flex", fontFamily: "'Georgia', serif", color: "#fff" }}>
      <style>{`
        @keyframes fadeIn { from { opacity: 0; transform: translateY(5px); } to { opacity: 1; transform: translateY(0); } }
        ::-webkit-scrollbar { width: 4px; }
        ::-webkit-scrollbar-thumb { background: #222; border-radius: 4px; }
      `}</style>

      {/* Sidebar */}
      <div style={{ width: 265, borderRight: "1px solid #1a1d2a", display: "flex", flexDirection: "column", flexShrink: 0 }}>
        <div style={{ padding: "28px 20px 16px", borderBottom: "1px solid #1a1d2a" }}>
          <div style={{ fontSize: 10, letterSpacing: 3, color: "#444", textTransform: "uppercase", fontFamily: "monospace" }}>Brevio</div>
          <div style={{ fontSize: 17, fontWeight: 700, marginTop: 4 }}>Message Copy</div>
          <div style={{ fontSize: 12, color: "#555", marginTop: 2 }}>Part 2 of 2</div>
        </div>

        {/* Section tabs */}
        <div style={{ display: "flex", flexDirection: "column", borderBottom: "1px solid #1a1d2a" }}>
          {Object.entries(sections).map(([key, s]) => (
            <button key={key} onClick={() => handleSectionChange(key)}
              style={{
                background: activeSection === key ? "#1a1d2a" : "transparent",
                border: "none", borderLeft: activeSection === key ? `3px solid ${s.color}` : "3px solid transparent",
                padding: "13px 18px", textAlign: "left", cursor: "pointer",
                color: activeSection === key ? "#fff" : "#555", transition: "all 0.15s",
              }}>
              <div style={{ fontSize: 16 }}>{s.icon} <span style={{ fontSize: 13, fontWeight: 600 }}>{s.label}</span></div>
              <div style={{ fontSize: 11, color: "#555", marginTop: 2 }}>{s.description}</div>
            </button>
          ))}
        </div>

        {/* Scenario list */}
        <div style={{ flex: 1, overflowY: "auto" }}>
          {sec.scenarios.map(s => (
            <button key={s.id} onClick={() => setActiveScenario(s.id)}
              style={{
                width: "100%", background: activeScenario === s.id ? "#1a1d2a" : "transparent",
                border: "none", borderLeft: activeScenario === s.id ? `3px solid ${sec.color}` : "3px solid transparent",
                padding: "11px 18px", textAlign: "left", cursor: "pointer",
                color: activeScenario === s.id ? "#fff" : "#555", transition: "all 0.15s",
                fontSize: 13,
              }}>
              {s.label}
              <div style={{ fontSize: 11, color: "#444", marginTop: 2 }}>{s.description}</div>
            </button>
          ))}
        </div>

        <div style={{ padding: "12px 18px", borderTop: "1px solid #1a1d2a", fontSize: 11, color: "#444", fontFamily: "monospace" }}>
          {sec.scenarios.length} scenarios in this section
        </div>
      </div>

      {/* Main */}
      <div style={{ flex: 1, display: "flex", flexDirection: "column", overflow: "hidden" }}>
        {/* Header */}
        <div style={{ padding: "22px 32px", borderBottom: "1px solid #1a1d2a", display: "flex", alignItems: "center", gap: 12 }}>
          <div style={{ width: 8, height: 8, borderRadius: "50%", background: sec.color, boxShadow: `0 0 10px ${sec.color}` }} />
          <div>
            <div style={{ fontSize: 17, fontWeight: 700 }}>{scenario.label}</div>
            <div style={{ fontSize: 12, color: "#555", marginTop: 2 }}>{scenario.description}</div>
          </div>
          <div style={{
            marginLeft: "auto", background: `${sec.color}18`, color: sec.color,
            fontSize: 10, padding: "5px 12px", borderRadius: 20,
            fontFamily: "monospace", border: `1px solid ${sec.color}33`,
          }}>{sec.label}</div>
        </div>

        {/* Content */}
        <div style={{ flex: 1, overflowY: "auto", padding: "28px 32px", display: "flex", justifyContent: "center" }}>
          <div style={{ width: "100%", maxWidth: 520, animation: "fadeIn 0.2s ease both" }}>

            {/* Phone */}
            <div style={{
              background: "#111318", borderRadius: 24,
              border: "1px solid #1a1d2a", overflow: "hidden",
              boxShadow: "0 20px 60px rgba(0,0,0,0.5)",
            }}>
              <div style={{
                background: "#16191f", padding: "13px 16px",
                display: "flex", alignItems: "center", gap: 10,
                borderBottom: "1px solid #1a1d2a",
              }}>
                <div style={{
                  width: 30, height: 30, borderRadius: "50%",
                  background: `linear-gradient(135deg, ${sec.color}99, #4F86F7)`,
                  display: "flex", alignItems: "center", justifyContent: "center",
                  fontWeight: 700, fontSize: 13,
                }}>B</div>
                <div>
                  <div style={{ fontSize: 13, fontWeight: 700 }}>Brevio</div>
                  <div style={{ fontSize: 10, color: "#4FF78A" }}>● Online</div>
                </div>
                <div style={{ marginLeft: "auto", fontSize: 11, color: "#444", fontFamily: "monospace" }}>
                  {scenario.messages.length} msg{scenario.messages.length !== 1 ? "s" : ""}
                </div>
              </div>
              <div style={{ padding: "18px 14px", minHeight: 160 }}>
                {scenario.messages.map((msg, i) => <ChatBubble key={i} msg={msg} />)}
              </div>
            </div>

            {/* Copy note */}
            <div style={{
              marginTop: 18, padding: "14px 18px",
              background: "#1a1d2a", borderRadius: 12,
              borderLeft: `3px solid ${sec.color}`,
            }}>
              <div style={{ fontSize: 10, color: sec.color, fontFamily: "monospace", textTransform: "uppercase", letterSpacing: 1, marginBottom: 6 }}>
                ✏️ Copy principle
              </div>
              <div style={{ fontSize: 13, color: "#aaa", lineHeight: 1.65 }}>{scenario.note}</div>
            </div>

            {/* Prev / Next */}
            <div style={{ display: "flex", justifyContent: "space-between", marginTop: 14 }}>
              {(() => {
                const idx = sec.scenarios.findIndex(s => s.id === activeScenario);
                const prev = sec.scenarios[idx - 1];
                const next = sec.scenarios[idx + 1];
                return (<>
                  <button onClick={() => prev && setActiveScenario(prev.id)} disabled={!prev}
                    style={{ background: prev ? "#1a1d2a" : "transparent", border: "none", color: prev ? "#ccc" : "#333", padding: "7px 14px", borderRadius: 8, cursor: prev ? "pointer" : "not-allowed", fontSize: 12 }}>
                    ← {prev?.label || ""}
                  </button>
                  <button onClick={() => next && setActiveScenario(next.id)} disabled={!next}
                    style={{ background: next ? sec.color : "transparent", border: "none", color: next ? "#000" : "#333", padding: "7px 14px", borderRadius: 8, cursor: next ? "pointer" : "not-allowed", fontSize: 12, fontWeight: next ? 600 : 400 }}>
                    {next?.label || ""} →
                  </button>
                </>);
              })()}
            </div>
          </div>
        </div>
      </div>
    </div>
  );
}
