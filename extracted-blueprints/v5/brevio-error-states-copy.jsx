import { useState } from "react";

const scenarios = [
  {
    id: "silent_1h",
    category: "Silent User",
    label: "Gone quiet — 1 hour",
    description: "User stopped responding mid-onboarding",
    color: "#F7C94F",
    messages: [
      { from: "brevio", text: "Hey, still there? 👋\n\nNo rush at all — whenever you're ready, just reply and we'll pick up right where we left off.", tag: null },
    ],
    note: "Soft nudge only. No guilt, no pressure. One message, then wait.",
  },
  {
    id: "silent_24h",
    category: "Silent User",
    label: "Gone quiet — 24 hours",
    description: "User hasn't responded in a full day",
    color: "#F7C94F",
    messages: [
      { from: "brevio", text: "Hey Albert! Just checking in — looks like we got interrupted yesterday. 😊\n\nWe were right in the middle of setting things up. Want to jump back in? Just say *\"resume\"* and I'll pick up from where we left off.\n\nOr if you'd rather start fresh, just say *\"restart\"*.", tag: null },
    ],
    note: "Give them a clear, low-effort action. 'Resume' or 'restart' — both feel safe.",
  },
  {
    id: "silent_72h",
    category: "Silent User",
    label: "Gone quiet — 72 hours",
    description: "User has been silent for 3 days",
    color: "#F7C94F",
    messages: [
      { from: "brevio", text: "Haven't heard from you in a few days — totally okay! Life gets busy.\n\nI'm still here whenever you need me. You don't have to finish setup to start using me — just ask me anything and I'll do my best.\n\nWhat's on your mind?", tag: null },
    ],
    note: "Drop the onboarding framing entirely. Pivot to value. Make it feel like setup doesn't matter — they can just use it.",
  },
  {
    id: "oauth_fail",
    category: "OAuth & Connections",
    label: "OAuth connection failed",
    description: "User tried to connect Google/Outlook but it didn't go through",
    color: "#F7824F",
    messages: [
      { from: "user", text: "Done!" },
      { from: "brevio", text: "Hmm, it looks like the connection didn't go through — I'm not seeing your account yet. 🤔\n\nHappens sometimes! A couple of things to try:\n\n1️⃣ Make sure you completed the authorization on Google's side (look for a green checkmark)\n2️⃣ Try the link again: brevio.app/connect/google\n\nIf it keeps not working, just say *\"skip for now\"* and we can connect it later.", tag: "Retry prompt" },
    ],
    note: "Never blame the user. Give them a way forward AND a way out. Always offer 'skip for now'.",
  },
  {
    id: "oauth_timeout",
    category: "OAuth & Connections",
    label: "OAuth link expired",
    description: "The connection link timed out before the user clicked it",
    color: "#F7824F",
    messages: [
      { from: "brevio", text: "Hey — that connection link expired (they only last 10 minutes for security). No worries!\n\nHere's a fresh one: brevio.app/connect/google\n\nTake your time — just don't close it halfway through or it'll need another refresh. 😄", tag: "Fresh link" },
    ],
    note: "Explain WHY it expired briefly — builds trust. Keep it light.",
  },
  {
    id: "unrecognized_input",
    category: "Unexpected Responses",
    label: "User sends something unexpected",
    description: "User replies with something Brevio can't map to an onboarding answer",
    color: "#7C6AF7",
    messages: [
      { from: "user", text: "Hmm I'm not sure about that one" },
      { from: "brevio", text: "Totally fine — no wrong answers here! 😊\n\nLet me rephrase:\n\n*How comfortable are you with me taking action on your behalf without checking in first?*\n\n1️⃣ Always ask me first\n2️⃣ Ask for big things, just do small stuff\n3️⃣ Use your judgment — I trust you\n\nOr just tell me in your own words — I'll figure it out.", tag: "Rephrased question" },
    ],
    note: "Rephrase, don't repeat. Always add the 'your own words' escape hatch.",
  },
  {
    id: "off_topic",
    category: "Unexpected Responses",
    label: "User goes off-topic mid-setup",
    description: "User asks Brevio to do something during onboarding",
    color: "#7C6AF7",
    messages: [
      { from: "user", text: "Actually wait — can you check what the weather is in Miami right now?" },
      { from: "brevio", text: "🌤️ Miami right now: 81°F, partly cloudy. Feels like 85°F.\n\n---\n\nOkay, back to getting you set up! We were on question 3 of 5:\n\n*How comfortable are you with me taking action on your behalf without checking in first?*\n\n1️⃣ Always ask me first\n2️⃣ Ask for big things, just do small stuff\n3️⃣ Use your judgment — I trust you", tag: "Task + resume" },
    ],
    note: "Just answer the question. Then resume. Don't make them feel bad for going off-track — it's a feature, not a bug.",
  },
  {
    id: "task_fail_generic",
    category: "Task Failures",
    label: "Task failed — generic",
    description: "Brevio attempted an action and it didn't work",
    color: "#F74F4F",
    messages: [
      { from: "brevio", text: "Something went wrong on my end when I tried to do that — sorry about that. 😕\n\nI've logged it so the team can look into it. In the meantime:\n\n• You can try again by sending the same message\n• Or I can try a different approach — just say *\"try another way\"*\n\nWhat would you like to do?", tag: "Generic failure" },
    ],
    note: "Own it. Don't say 'there was an error' — say I tried and it didn't work. Give next steps, not just an apology.",
  },
  {
    id: "task_fail_permission",
    category: "Task Failures",
    label: "Task failed — missing permission",
    description: "Brevio can't do what the user asked because a service isn't connected",
    color: "#F74F4F",
    messages: [
      { from: "user", text: "Can you reschedule my 3pm meeting tomorrow?" },
      { from: "brevio", text: "I'd love to! But I don't have access to your calendar yet.\n\nTakes about 30 seconds to connect:\n🔗 brevio.app/connect/google\n\nOnce you do, I can handle rescheduling, invites, conflicts — all of it.", tag: "Permission prompt" },
    ],
    note: "Frame the missing permission as unlocking a feature, not an error. Lead with what they'll gain.",
  },
  {
    id: "task_fail_approval",
    category: "Task Failures",
    label: "Task blocked — needs approval",
    description: "Brevio wants to take an action but user's autonomy settings require sign-off",
    color: "#F74F4F",
    messages: [
      { from: "brevio", text: "Before I do this, I want to check in — you asked me to *send an email to Marcus confirming the Friday deadline*.\n\nHere's what I'd send:\n\n---\n*\"Hi Marcus, confirming I'll have the contract reviewed by Friday EOD. Thanks for your patience.\"*\n---\n\nSend it?\n\n✅ *\"Yes, send it\"*\n✏️ *\"Edit first\"*\n❌ *\"Cancel\"*", tag: "Approval gate" },
    ],
    note: "Show exactly what will happen before it happens. Never summarize — show the actual content. Give edit + cancel, not just yes/no.",
  },
  {
    id: "rate_limit",
    category: "System Limits",
    label: "Rate limit / too many requests",
    description: "User is sending messages faster than the system can process",
    color: "#4FC9F7",
    messages: [
      { from: "brevio", text: "Whoa, you're fast! 😄 I'm still working through your last request — give me just a moment.\n\nI'll reply as soon as I'm done.", tag: "Rate limit" },
    ],
    note: "Keep it light. Never say 'rate limit' or anything technical. Make it feel like you're just busy, not broken.",
  },
  {
    id: "llm_timeout",
    category: "System Limits",
    label: "Response taking too long",
    description: "Brevio is taking longer than expected to respond",
    color: "#4FC9F7",
    messages: [
      { from: "brevio", text: "Still working on this — it's a bit more involved than usual. Hang tight... ⏳" },
      { from: "brevio", text: "Done! Here's what I found:", tag: "Delay + resolution" },
    ],
    note: "Send a progress message if response will take >5s. Never go silent. A 'still working' message dramatically reduces perceived wait time.",
  },
];

const categories = [...new Set(scenarios.map(s => s.category))];
const categoryColors = {
  "Silent User": "#F7C94F",
  "OAuth & Connections": "#F7824F",
  "Unexpected Responses": "#7C6AF7",
  "Task Failures": "#F74F4F",
  "System Limits": "#4FC9F7",
};

function ChatBubble({ msg }) {
  const isBrevio = msg.from === "brevio";
  return (
    <div style={{ display: "flex", flexDirection: "column", alignItems: isBrevio ? "flex-start" : "flex-end", marginBottom: 10 }}>
      <div style={{ display: "flex", alignItems: "flex-end", gap: 8, flexDirection: isBrevio ? "row" : "row-reverse" }}>
        {isBrevio && (
          <div style={{
            width: 26, height: 26, borderRadius: "50%",
            background: "linear-gradient(135deg, #4F86F7, #7C6AF7)",
            display: "flex", alignItems: "center", justifyContent: "center",
            fontSize: 12, fontWeight: 700, flexShrink: 0,
          }}>B</div>
        )}
        <div style={{
          maxWidth: "78%",
          padding: "10px 14px",
          borderRadius: isBrevio ? "18px 18px 18px 4px" : "18px 18px 4px 18px",
          background: isBrevio ? "#1a1d2a" : "#4F86F7",
          color: "#fff",
          fontSize: 13.5,
          lineHeight: 1.6,
          whiteSpace: "pre-wrap",
          boxShadow: "0 2px 8px rgba(0,0,0,0.3)",
        }}
          dangerouslySetInnerHTML={{
            __html: msg.text.replace(/\*(.*?)\*/g, "<strong>$1</strong>")
          }}
        />
      </div>
      {msg.tag && (
        <div style={{
          marginTop: 4,
          marginLeft: isBrevio ? 34 : 0,
          fontSize: 10,
          color: "#555",
          fontFamily: "monospace",
          letterSpacing: 1,
          textTransform: "uppercase",
        }}>
          {msg.tag}
        </div>
      )}
    </div>
  );
}

export default function BrevioErrorStates() {
  const [activeId, setActiveId] = useState(scenarios[0].id);
  const [activeCategory, setActiveCategory] = useState(null);

  const filtered = activeCategory
    ? scenarios.filter(s => s.category === activeCategory)
    : scenarios;

  const active = scenarios.find(s => s.id === activeId) || scenarios[0];

  return (
    <div style={{
      minHeight: "100vh",
      background: "#0b0d14",
      display: "flex",
      fontFamily: "'Georgia', serif",
      color: "#fff",
    }}>
      <style>{`
        @keyframes fadeIn { from { opacity: 0; transform: translateY(6px); } to { opacity: 1; transform: translateY(0); } }
        ::-webkit-scrollbar { width: 4px; }
        ::-webkit-scrollbar-thumb { background: #222; border-radius: 4px; }
        button:hover { opacity: 0.85; }
      `}</style>

      {/* Sidebar */}
      <div style={{
        width: 270,
        borderRight: "1px solid #1a1d2a",
        display: "flex",
        flexDirection: "column",
        flexShrink: 0,
        overflow: "hidden",
      }}>
        {/* Title */}
        <div style={{ padding: "28px 20px 20px", borderBottom: "1px solid #1a1d2a" }}>
          <div style={{ fontSize: 10, letterSpacing: 3, color: "#444", textTransform: "uppercase", fontFamily: "monospace" }}>Brevio</div>
          <div style={{ fontSize: 17, fontWeight: 700, marginTop: 4 }}>Error States</div>
          <div style={{ fontSize: 12, color: "#555", marginTop: 3 }}>& Silent User Recovery</div>
        </div>

        {/* Category filters */}
        <div style={{ padding: "12px 16px", borderBottom: "1px solid #1a1d2a", display: "flex", flexWrap: "wrap", gap: 6 }}>
          <button
            onClick={() => setActiveCategory(null)}
            style={{
              background: !activeCategory ? "#fff" : "#1a1d2a",
              color: !activeCategory ? "#000" : "#777",
              border: "none", borderRadius: 20, padding: "4px 10px",
              fontSize: 11, cursor: "pointer", fontFamily: "monospace",
            }}
          >All</button>
          {categories.map(cat => (
            <button key={cat}
              onClick={() => setActiveCategory(activeCategory === cat ? null : cat)}
              style={{
                background: activeCategory === cat ? categoryColors[cat] : "#1a1d2a",
                color: activeCategory === cat ? "#000" : "#777",
                border: "none", borderRadius: 20, padding: "4px 10px",
                fontSize: 11, cursor: "pointer", fontFamily: "monospace",
              }}
            >{cat}</button>
          ))}
        </div>

        {/* Scenario list */}
        <div style={{ flex: 1, overflowY: "auto" }}>
          {filtered.map(s => (
            <button key={s.id} onClick={() => setActiveId(s.id)}
              style={{
                width: "100%", background: activeId === s.id ? "#1a1d2a" : "transparent",
                border: "none", borderLeft: activeId === s.id ? `3px solid ${s.color}` : "3px solid transparent",
                padding: "12px 16px", textAlign: "left", cursor: "pointer",
                color: activeId === s.id ? "#fff" : "#555",
                transition: "all 0.15s",
              }}
            >
              <div style={{ fontSize: 10, color: s.color, fontFamily: "monospace", marginBottom: 3, textTransform: "uppercase", letterSpacing: 0.5 }}>{s.category}</div>
              <div style={{ fontSize: 13 }}>{s.label}</div>
            </button>
          ))}
        </div>

        {/* Count */}
        <div style={{ padding: "12px 16px", borderTop: "1px solid #1a1d2a", fontSize: 11, color: "#444", fontFamily: "monospace" }}>
          {filtered.length} scenarios · {scenarios.length} total
        </div>
      </div>

      {/* Main panel */}
      <div style={{ flex: 1, display: "flex", flexDirection: "column", overflow: "hidden" }}>

        {/* Header */}
        <div style={{ padding: "24px 32px", borderBottom: "1px solid #1a1d2a", display: "flex", alignItems: "center", gap: 14 }}>
          <div style={{ width: 8, height: 8, borderRadius: "50%", background: active.color, boxShadow: `0 0 10px ${active.color}` }} />
          <div>
            <div style={{ fontSize: 17, fontWeight: 700 }}>{active.label}</div>
            <div style={{ fontSize: 12, color: "#555", marginTop: 2 }}>{active.description}</div>
          </div>
          <div style={{
            marginLeft: "auto",
            background: "#1a1d2a",
            borderRadius: 8,
            padding: "6px 14px",
            fontSize: 11,
            color: active.color,
            fontFamily: "monospace",
            letterSpacing: 0.5,
          }}>{active.category}</div>
        </div>

        {/* Content */}
        <div style={{ flex: 1, overflowY: "auto", padding: "32px", display: "flex", justifyContent: "center" }}>
          <div style={{ width: "100%", maxWidth: 520, animation: "fadeIn 0.25s ease both" }}>

            {/* Phone mockup */}
            <div style={{
              background: "#111318",
              borderRadius: 24,
              border: "1px solid #1a1d2a",
              overflow: "hidden",
              boxShadow: "0 20px 70px rgba(0,0,0,0.5)",
            }}>
              {/* Phone bar */}
              <div style={{
                background: "#16191f",
                padding: "14px 18px",
                display: "flex",
                alignItems: "center",
                gap: 10,
                borderBottom: "1px solid #1a1d2a",
              }}>
                <div style={{
                  width: 32, height: 32, borderRadius: "50%",
                  background: `linear-gradient(135deg, ${active.color}99, #4F86F7)`,
                  display: "flex", alignItems: "center", justifyContent: "center",
                  fontWeight: 700, fontSize: 14,
                }}>B</div>
                <div>
                  <div style={{ fontSize: 13, fontWeight: 700 }}>Brevio</div>
                  <div style={{ fontSize: 10, color: "#4FF78A" }}>● Online</div>
                </div>
                <div style={{
                  marginLeft: "auto",
                  background: `${active.color}22`,
                  color: active.color,
                  fontSize: 10,
                  padding: "3px 10px",
                  borderRadius: 20,
                  fontFamily: "monospace",
                  border: `1px solid ${active.color}44`,
                }}>
                  {active.messages.length} msg{active.messages.length !== 1 ? "s" : ""}
                </div>
              </div>

              {/* Messages */}
              <div style={{ padding: "20px 16px", minHeight: 180 }}>
                {active.messages.map((msg, i) => (
                  <ChatBubble key={i} msg={msg} />
                ))}
              </div>
            </div>

            {/* Writer's note */}
            <div style={{
              marginTop: 20,
              padding: "16px 20px",
              background: "#1a1d2a",
              borderRadius: 12,
              borderLeft: `3px solid ${active.color}`,
            }}>
              <div style={{ fontSize: 11, color: active.color, fontFamily: "monospace", textTransform: "uppercase", letterSpacing: 1, marginBottom: 8 }}>
                ✏️ Copy principle
              </div>
              <div style={{ fontSize: 13, color: "#aaa", lineHeight: 1.65 }}>
                {active.note}
              </div>
            </div>

            {/* Navigation */}
            <div style={{ display: "flex", justifyContent: "space-between", marginTop: 16 }}>
              {(() => {
                const idx = scenarios.findIndex(s => s.id === activeId);
                const prev = scenarios[idx - 1];
                const next = scenarios[idx + 1];
                return (
                  <>
                    <button onClick={() => prev && setActiveId(prev.id)}
                      disabled={!prev}
                      style={{
                        background: prev ? "#1a1d2a" : "transparent",
                        border: "none", color: prev ? "#fff" : "#333",
                        padding: "8px 16px", borderRadius: 8, cursor: prev ? "pointer" : "not-allowed",
                        fontSize: 13,
                      }}>
                      ← {prev?.label || ""}
                    </button>
                    <button onClick={() => next && setActiveId(next.id)}
                      disabled={!next}
                      style={{
                        background: next ? active.color : "transparent",
                        border: "none", color: next ? "#000" : "#333",
                        padding: "8px 16px", borderRadius: 8, cursor: next ? "pointer" : "not-allowed",
                        fontSize: 13, fontWeight: next ? 600 : 400,
                      }}>
                      {next?.label || ""} →
                    </button>
                  </>
                );
              })()}
            </div>
          </div>
        </div>
      </div>
    </div>
  );
}
