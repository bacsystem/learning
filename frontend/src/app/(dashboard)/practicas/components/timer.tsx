"use client";

import { useEffect, useRef, useState } from "react";

import { cn } from "@/lib/utils";
import { TimerIcon } from "@phosphor-icons/react";

interface TimerProps {
  /**
   * Seconds left as computed by the server clock (tiempo_restante_seg), or
   * null when the práctica is untimed. The deadline is derived from it, so a
   * reload does not restart the countdown (H82).
   */
  tiempoRestanteSeg: number | null;
  onExpired: () => void;
}

function formatTime(seconds: number): string {
  const m = Math.floor(seconds / 60);
  const s = seconds % 60;
  return `${String(m).padStart(2, "0")}:${String(s).padStart(2, "0")}`;
}

export function Timer({ tiempoRestanteSeg, onExpired }: TimerProps) {
  const deadlineRef = useRef<number | null>(null);
  const [remaining, setRemaining] = useState<number | null>(null);
  const expiredRef = useRef(false);
  // onExpired changes identity whenever the parent's state changes (e.g. each
  // "Comprobar"); keeping it in a ref lets the countdown effect depend only on
  // the server value, so the deadline is not reset on every re-render (H104).
  const onExpiredRef = useRef(onExpired);
  useEffect(() => {
    onExpiredRef.current = onExpired;
  }, [onExpired]);

  useEffect(() => {
    if (tiempoRestanteSeg === null) return;

    deadlineRef.current = Date.now() + tiempoRestanteSeg * 1000;
    setRemaining(tiempoRestanteSeg);

    const tick = () => {
      const rem = Math.max(0, Math.round((deadlineRef.current! - Date.now()) / 1000));
      setRemaining(rem);
      if (rem === 0 && !expiredRef.current) {
        expiredRef.current = true;
        clearInterval(interval);
        onExpiredRef.current();
      }
    };
    const interval = setInterval(tick, 1000);
    if (tiempoRestanteSeg === 0) tick();

    return () => clearInterval(interval);
  }, [tiempoRestanteSeg]);

  if (tiempoRestanteSeg === null || remaining === null) return null;

  const isUrgent = remaining < 60;

  return (
    <div
      role="timer"
      aria-label={formatTime(remaining)}
      className={cn(
        "h-control ease-salida inline-flex shrink-0 items-center gap-2 rounded-full px-3.5 transition-colors duration-(--duracion-media)",
        isUrgent ? "bg-destructive-suave text-destructive-texto" : "bg-card text-foreground ring-border ring-1",
      )}
    >
      <TimerIcon
        aria-hidden
        className={cn("size-4", isUrgent ? "motion-safe:animate-pulse" : "text-muted-foreground")}
      />
      <span className="text-titulo tabular-nums">{formatTime(remaining)}</span>
    </div>
  );
}
