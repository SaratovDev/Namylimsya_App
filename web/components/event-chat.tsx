"use client";

import { useState, useRef, useEffect } from "react";
import { Send } from "lucide-react";
import { useChat } from "@/lib/use-chat";
import { Avatar, AvatarFallback, AvatarImage } from "@/components/ui/avatar";
import { Input } from "@/components/ui/input";
import { Button } from "@/components/ui/button";

type Props = {
  eventId: string;
  token: string | null;
  currentUserId: string | null;
};

export function EventChat({ eventId, token, currentUserId }: Props) {
  const { messages, loading, connected, denied, lastError, sendMessage } =
    useChat(eventId, token);
  const [input, setInput] = useState("");
  const scrollRef = useRef<HTMLDivElement>(null);

  useEffect(() => {
    if (scrollRef.current) {
      scrollRef.current.scrollTop = scrollRef.current.scrollHeight;
    }
  }, [messages]);

  const handleSend = (e: React.FormEvent) => {
    e.preventDefault();
    const text = input.trim();
    if (!text) return;
    if (sendMessage(text)) {
      setInput("");
    }
  };

  if (loading) {
    return (
      <div className="h-[400px] flex items-center justify-center text-muted-foreground">
        Загрузка чата...
      </div>
    );
  }

  return (
    <div className="flex flex-col h-[500px] border rounded-xl overflow-hidden bg-card/5">
      <div ref={scrollRef} className="flex-1 overflow-y-auto p-4 space-y-4">
        {messages.length === 0 ? (
          <div className="text-center text-muted-foreground text-sm mt-10">
            Сообщений пока нет. Напишите первым!
          </div>
        ) : (
          messages.map((msg) => {
            const isMe = msg.user_id === currentUserId;
            return (
              <div
                key={msg.id}
                className={`flex gap-2 max-w-[85%] ${
                  isMe ? "ml-auto flex-row-reverse" : ""
                }`}
              >
                {!isMe && (
                  <Avatar className="h-8 w-8 mt-1">
                    <AvatarImage src={msg.user_avatar ?? ""} />
                    <AvatarFallback className="bg-primary/20 text-xs">
                      {msg.user_name?.slice(0, 1).toUpperCase() || "?"}
                    </AvatarFallback>
                  </Avatar>
                )}

                <div
                  className={`flex flex-col ${
                    isMe ? "items-end" : "items-start"
                  }`}
                >
                  {!isMe && (
                    <span className="text-[11px] font-medium text-muted-foreground mb-1 ml-1">
                      {msg.user_name}
                    </span>
                  )}

                  <div
                    className={`px-3 py-2 rounded-2xl text-sm ${
                      isMe
                        ? "bg-primary text-primary-foreground rounded-tr-sm"
                        : "bg-muted text-foreground rounded-tl-sm"
                    }`}
                  >
                    {msg.text}
                  </div>
                  <span className="text-[10px] text-muted-foreground mt-1 mx-1">
                    {new Date(msg.created_at).toLocaleTimeString("ru-RU", {
                      hour: "2-digit",
                      minute: "2-digit",
                    })}
                  </span>
                </div>
              </div>
            );
          })
        )}
      </div>

      {lastError && (
        <div className="px-3 py-2 text-xs text-destructive border-t bg-destructive/5">
          {lastError}
        </div>
      )}

      <div className="p-3 bg-background border-t">
        <form onSubmit={handleSend} className="flex gap-2">
          <Input
            value={input}
            onChange={(e) => setInput(e.target.value)}
            placeholder={
              denied
                ? "Чат доступен после одобрения заявки"
                : connected
                ? "Написать сообщение..."
                : "Подключение к чату..."
            }
            disabled={!connected || denied}
            className="flex-1 bg-muted/50 border-transparent focus-visible:bg-background"
          />
          <Button
            type="submit"
            size="icon"
            disabled={!connected || denied || !input.trim()}
          >
            <Send className="h-4 w-4" />
          </Button>
        </form>
      </div>
    </div>
  );
}