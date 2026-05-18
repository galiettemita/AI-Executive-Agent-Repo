import { useState } from "react";

const stages = [
  {
    id: "welcome",
    label: "Stage 1 — First Message",
    description: "User messages Brevio for the first time",
    messages: [
      {
        from: "brevio",
        text: "Hey! 👋 I'm so glad you're here.\n\nI'm Brevio — your personal AI assistant. I can help you manage your schedule, handle emails, book things, research anything, and a whole lot more. All right here in this chat.\n\nNo apps. No dashboards. Just message me like you would a friend.",
      },
      {
        from: "brevio",
        text: "Before I dive in, I'd love to set things up so I can actually be useful to you. It'll take about 2 minutes — and I'll make it worth it. 🙂\n\nLet's start simple: what's your name?",
      },
      { from: "user", text: "Albert" },
      {
        from: "brevio",
        text: "Great to meet you, Albert! Let's get you set up. ✨",
      },
    ],
  },
  {
    id: "discovery",
    label: "Stage 2 — Discovery (5 Questions)",
    description: "Brevio learns what the user needs",
    messages: [
      {
        from: "brevio",
        text: "I have 5 quick questions — just reply with a number or type your answer, whatever feels natural.\n\n*Question 1 of 5*\nWhat best describes you?\n\n1️⃣ I run a business or team\n2️⃣ I'm a busy professional\n3️⃣ I'm managing a packed personal life\n4️⃣ A mix of all of the above",
      },
      { from: "user", text: "4" },
      {
        from: "brevio",
        text: "Love it — the best kind of chaos 😄\n\n*Question 2 of 5*\nWhat eats up most of your time right now?\n\n1️⃣ Email & messages\n2️⃣ Scheduling & calendar\n3️⃣ Research & decisions\n4️⃣ Tasks & follow-ups\n5️⃣ All of the above, honestly",
      },
      { from: "user", text: "5" },
      {
        from: "brevio",
        text: "*Question 3 of 5*\nHow do you like to communicate?\n\n1️⃣ Keep it short and to the point\n2️⃣ Give me context and details\n3️⃣ Depends on the situation",
      },
      { from: "user", text: "3" },
      {
        from: "brevio",
        text: "*Question 4 of 5*\nHow comfortable are you with me taking action on your behalf — like sending a message or booking something — without checking in first?\n\n1️⃣ Always ask me first\n2️⃣ Ask for big things, just do small stuff\n3️⃣ Use your judgment — I trust you",
      },
      { from: "user", text: "2" },
      {
        from: "brevio",
        text: "*Question 5 of 5*\nWhat would make you feel like I'm actually earning my keep? What's the one thing, if I nailed it, that would make you think \"okay, this is worth it\"?",
      },
      { from: "user", text: "Managing my inbox without me having to think about it" },
      {
        from: "brevio",
        text: "Got it. Inbox zero, on autopilot. I've got you. 📬\n\nI've saved all of this — I'll use it to shape how I work with you going forward.",
      },
    ],
  },
  {
    id: "oauth",
    label: "Stage 3 — Connect Services",
    description: "Brevio invites the user to connect their tools",
    messages: [
      {
        from: "brevio",
        text: "To actually help with your inbox and calendar, I'll need to connect to your accounts.\n\nWhich of these do you use? (Reply with the numbers, or say \"skip\" to do this later)\n\n1️⃣ Gmail\n2️⃣ Google Calendar\n3️⃣ Outlook / Microsoft 365\n4️⃣ Slack\n5️⃣ Something else",
      },
      { from: "user", text: "1, 2" },
      {
        from: "brevio",
        text: "Perfect. Here's your secure link to connect Google:\n\n🔗 brevio.app/connect/google\n\nTakes about 30 seconds. I'll wait right here. 🙂",
      },
      { from: "user", text: "Done!" },
      {
        from: "brevio",
        text: "✅ Connected! I can see your Gmail and Calendar now.\n\nYou can connect more services anytime — just ask me. Want to add anything else now, or should we move on?",
      },
      { from: "user", text: "Let's move on" },
    ],
  },
  {
    id: "calibration",
    label: "Stage 4 — Preference Calibration",
    description: "Fine-tuning how Brevio communicates",
    messages: [
      {
        from: "brevio",
        text: "Almost there! Just a couple of quick preferences so I communicate the way *you* like.\n\nHow often should I check in with you proactively?\n\n1️⃣ Only when it's urgent\n2️⃣ Daily summary is enough\n3️⃣ Keep me in the loop throughout the day",
      },
      { from: "user", text: "2" },
      {
        from: "brevio",
        text: "Got it — I'll send you a daily briefing each morning.\n\nAnd when something needs your approval, how do you want me to present it?\n\n1️⃣ Just the bottom line — what do I need to decide?\n2️⃣ Give me a bit of context first\n3️⃣ Full picture every time",
      },
      { from: "user", text: "1" },
      {
        from: "brevio",
        text: "Perfect. Short, sharp, and actionable. My kind of vibe. ✅",
      },
    ],
  },
  {
    id: "first_value",
    label: "Stage 5 — First Value Moment",
    description: "Brevio delivers immediate value",
    messages: [
      {
        from: "brevio",
        text: "You're all set, Albert! 🎉\n\nI took a quick peek at your inbox while we were talking...\n\nYou have *14 unread emails* from the last 24 hours. I've sorted them:\n\n📌 *3 need your attention today*\n📋 *7 are FYI — I'll summarize them*\n🗑️ *4 are noise — I'd suggest archiving them*\n\nWant me to walk you through the 3 important ones?",
      },
      { from: "user", text: "Yes please" },
      {
        from: "brevio",
        text: "*1 of 3 — Action needed*\n📧 From: Marcus Chen\nSubject: Contract review — deadline Friday\n\nMarcus sent the revised contract and is waiting on your sign-off before Friday EOD. He flagged Section 4 as the main change.\n\nWhat would you like to do?\n1️⃣ Draft a reply for me to review\n2️⃣ Remind me Thursday morning\n3️⃣ I'll handle it myself",
      },
    ],
  },
  {
    id: "complete",
    label: "Stage 6 — Wrap Up",
    description: "Closing the onboarding loop",
    messages: [
      {
        from: "brevio",
        text: "That's what I'm here for. 💪\n\nA few things to know going forward:\n\n• *Just talk to me naturally* — no commands or special syntax needed\n• *I'll send your morning briefing* at 8am starting tomorrow\n• *You're in control* — you can always change my settings by saying \"update my preferences\"\n\nWhat else can I help you with today?",
      },
    ],
  },
];

const stageColors = {
  welcome: "#4F86F7",
  discovery: "#7C6AF7",
  oauth: "#F7824F",
  calibration: "#4FC9F7",
  first_value: "#4FF78A",
  complete: "#F7C94F",
};

function ChatBubble({ msg, animate }) {
  const isBrevio = msg.from === "brevio";
  return (
    <div
      style={{
        display: "flex",
        justifyContent: isBrevio ? "flex-start" : "flex-end",
        marginBottom: "8px",
        animation: animate ? "fadeUp 0.3s ease both" : "none",
      }}
    >
      {isBrevio && (
        <div
          style={{
            width: 28,
            height: 28,
            borderRadius: "50%",
            background: "linear-gradient(135deg, #4F86F7, #7C6AF7)",
            display: "flex",
            alignItems: "center",
            justifyContent: "center",
            fontSize: 13,
            marginRight: 8,
            flexShrink: 0,
            alignSelf: "flex-end",
          }}
        >
          B
        </div>
      )}
      <div
        style={{
          maxWidth: "75%",
          padding: "10px 14px",
          borderRadius: isBrevio ? "18px 18px 18px 4px" : "18px 18px 4px 18px",
          background: isBrevio ? "#1e2130" : "#4F86F7",
          color: "#fff",
          fontSize: 14,
          lineHeight: 1.55,
          whiteSpace: "pre-wrap",
          boxShadow: "0 1px 4px rgba(0,0,0,0.3)",
        }}
        dangerouslySetInnerHTML={{
          __html: msg.text
            .replace(/\*(.*?)\*/g, "<strong>$1</strong>")
            .replace(/📧/g, "📧")
        }}
      />
    </div>
  );
}

export default function BrevioOnboarding() {
  const [activeStage, setActiveStage] = useState(0);
  const stage = stages[activeStage];

  return (
    <div
      style={{
        minHeight: "100vh",
        background: "#0b0d14",
        display: "flex",
        fontFamily: "'Georgia', serif",
        color: "#fff",
      }}
    >
      <style>{`
        @keyframes fadeUp {
          from { opacity: 0; transform: translateY(8px); }
          to { opacity: 1; transform: translateY(0); }
        }
        ::-webkit-scrollbar { width: 4px; }
        ::-webkit-scrollbar-thumb { background: #333; border-radius: 4px; }
      `}</style>

      {/* Sidebar */}
      <div
        style={{
          width: 260,
          borderRight: "1px solid #1e2130",
          padding: "32px 0",
          display: "flex",
          flexDirection: "column",
          gap: 4,
          flexShrink: 0,
        }}
      >
        <div style={{ padding: "0 24px 24px", borderBottom: "1px solid #1e2130" }}>
          <div style={{ fontSize: 11, letterSpacing: 3, color: "#555", textTransform: "uppercase", fontFamily: "monospace" }}>
            Brevio
          </div>
          <div style={{ fontSize: 18, fontWeight: 700, marginTop: 4 }}>
            Onboarding Copy
          </div>
          <div style={{ fontSize: 12, color: "#666", marginTop: 4 }}>
            WhatsApp / iMessage flows
          </div>
        </div>

        {stages.map((s, i) => (
          <button
            key={s.id}
            onClick={() => setActiveStage(i)}
            style={{
              background: activeStage === i ? "#1e2130" : "transparent",
              border: "none",
              borderLeft: activeStage === i ? `3px solid ${stageColors[s.id]}` : "3px solid transparent",
              padding: "12px 24px",
              textAlign: "left",
              cursor: "pointer",
              color: activeStage === i ? "#fff" : "#666",
              transition: "all 0.15s",
            }}
          >
            <div style={{ fontSize: 11, color: stageColors[s.id], fontFamily: "monospace", marginBottom: 2 }}>
              {s.label.split("—")[0].trim()}
            </div>
            <div style={{ fontSize: 13 }}>
              {s.label.split("—")[1]?.trim()}
            </div>
          </button>
        ))}
      </div>

      {/* Main */}
      <div style={{ flex: 1, display: "flex", flexDirection: "column", overflow: "hidden" }}>

        {/* Header */}
        <div
          style={{
            padding: "24px 32px",
            borderBottom: "1px solid #1e2130",
            display: "flex",
            alignItems: "center",
            gap: 16,
          }}
        >
          <div
            style={{
              width: 8,
              height: 8,
              borderRadius: "50%",
              background: stageColors[stage.id],
              boxShadow: `0 0 12px ${stageColors[stage.id]}`,
            }}
          />
          <div>
            <div style={{ fontSize: 17, fontWeight: 700 }}>{stage.label}</div>
            <div style={{ fontSize: 13, color: "#666", marginTop: 2 }}>{stage.description}</div>
          </div>

          <div style={{ marginLeft: "auto", display: "flex", gap: 8 }}>
            <button
              onClick={() => setActiveStage(Math.max(0, activeStage - 1))}
              disabled={activeStage === 0}
              style={{
                background: "#1e2130",
                border: "none",
                color: activeStage === 0 ? "#333" : "#fff",
                padding: "8px 16px",
                borderRadius: 8,
                cursor: activeStage === 0 ? "not-allowed" : "pointer",
                fontSize: 13,
              }}
            >
              ← Prev
            </button>
            <button
              onClick={() => setActiveStage(Math.min(stages.length - 1, activeStage + 1))}
              disabled={activeStage === stages.length - 1}
              style={{
                background: stageColors[stage.id],
                border: "none",
                color: "#fff",
                padding: "8px 16px",
                borderRadius: 8,
                cursor: activeStage === stages.length - 1 ? "not-allowed" : "pointer",
                fontSize: 13,
                fontWeight: 600,
              }}
            >
              Next →
            </button>
          </div>
        </div>

        {/* Chat Area */}
        <div style={{ flex: 1, overflowY: "auto", padding: "32px", display: "flex", justifyContent: "center" }}>
          <div style={{ width: "100%", maxWidth: 540 }}>

            {/* Phone mockup */}
            <div
              style={{
                background: "#111318",
                borderRadius: 24,
                border: "1px solid #1e2130",
                overflow: "hidden",
                boxShadow: "0 24px 80px rgba(0,0,0,0.6)",
              }}
            >
              {/* Phone header */}
              <div
                style={{
                  background: "#16191f",
                  padding: "16px 20px",
                  display: "flex",
                  alignItems: "center",
                  gap: 12,
                  borderBottom: "1px solid #1e2130",
                }}
              >
                <div
                  style={{
                    width: 36,
                    height: 36,
                    borderRadius: "50%",
                    background: `linear-gradient(135deg, ${stageColors[stage.id]}, #4F86F7)`,
                    display: "flex",
                    alignItems: "center",
                    justifyContent: "center",
                    fontSize: 16,
                    fontWeight: 700,
                  }}
                >
                  B
                </div>
                <div>
                  <div style={{ fontSize: 14, fontWeight: 700 }}>Brevio</div>
                  <div style={{ fontSize: 11, color: "#4FF78A", display: "flex", alignItems: "center", gap: 4 }}>
                    <span style={{ width: 6, height: 6, borderRadius: "50%", background: "#4FF78A", display: "inline-block" }} />
                    Online
                  </div>
                </div>
              </div>

              {/* Messages */}
              <div style={{ padding: "20px 16px", minHeight: 300 }}>
                {stage.messages.map((msg, i) => (
                  <ChatBubble key={i} msg={msg} animate={true} />
                ))}
              </div>
            </div>

            {/* Notes */}
            <div
              style={{
                marginTop: 24,
                padding: "16px 20px",
                background: "#1e2130",
                borderRadius: 12,
                fontSize: 13,
                color: "#888",
                lineHeight: 1.6,
                borderLeft: `3px solid ${stageColors[stage.id]}`,
              }}
            >
              <div style={{ color: "#bbb", fontWeight: 600, marginBottom: 6 }}>✏️ Copy notes</div>
              {stage.id === "welcome" && "Hook fast — the first message sets the entire relationship. No feature lists, just personality and a single ask."}
              {stage.id === "discovery" && "5 questions max. Each one builds the USER.md profile used by Brain for all future interactions. Keep choices scannable — numbers, not paragraphs."}
              {stage.id === "oauth" && "Frame OAuth as enabling their goal (inbox), not as a technical step. Always offer a skip path. Confirm immediately after connection."}
              {stage.id === "calibration" && "Only ask what actually changes behavior in the system. Don't ask questions you won't act on. 2 questions is enough here."}
              {stage.id === "first_value" && "This is the most important moment. Brevio should already have done something useful BEFORE the user asks. Lead with output, not setup."}
              {stage.id === "complete" && "Don't end with 'let me know if you need anything.' End with openness, not closure. Leave the door obviously open."}
            </div>
          </div>
        </div>

        {/* Footer: stage count */}
        <div
          style={{
            padding: "16px 32px",
            borderTop: "1px solid #1e2130",
            display: "flex",
            alignItems: "center",
            justifyContent: "space-between",
            fontSize: 12,
            color: "#555",
          }}
        >
          <span>{stage.messages.length} messages in this stage</span>
          <div style={{ display: "flex", gap: 6 }}>
            {stages.map((s, i) => (
              <div
                key={s.id}
                onClick={() => setActiveStage(i)}
                style={{
                  width: i === activeStage ? 20 : 6,
                  height: 6,
                  borderRadius: 3,
                  background: i === activeStage ? stageColors[s.id] : "#2a2d3a",
                  cursor: "pointer",
                  transition: "all 0.2s",
                }}
              />
            ))}
          </div>
          <span>Stage {activeStage + 1} of {stages.length}</span>
        </div>
      </div>
    </div>
  );
}
