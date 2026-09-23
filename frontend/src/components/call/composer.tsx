"use client";

import { useState } from "react";
import { SendHorizonal } from "lucide-react";
import { Button } from "@/components/ui/button";

export function Composer({ onSend, disabled }: { onSend: (text: string) => void; disabled?: boolean }) {
  const [text, setText] = useState("");
  const submit = () => {
    const t = text.trim();
    if (!t) return;
    onSend(t);
    setText("");
  };
  return (
    <form
      className="flex items-center gap-2"
      onSubmit={(e) => {
        e.preventDefault();
        submit();
      }}
    >
      <input
        value={text}
        onChange={(e) => setText(e.target.value)}
        placeholder="Type a message in Russian or Kazakh…"
        aria-label="Message"
        disabled={disabled}
        className="h-9 min-w-0 flex-1 rounded-[10px] border border-input bg-background/60 px-3 text-[13px] outline-none transition-colors placeholder:text-muted-foreground/50 focus-visible:border-primary/60 focus-visible:ring-2 focus-visible:ring-primary/25 disabled:opacity-50"
      />
      <Button type="submit" variant="primary" size="lg" disabled={disabled || !text.trim()} aria-label="Send">
        <SendHorizonal className="size-4" /> Send
      </Button>
    </form>
  );
}
