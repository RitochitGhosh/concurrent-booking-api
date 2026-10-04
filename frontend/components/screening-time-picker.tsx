"use client";

import { useId, useRef, useState } from "react";
import { CalendarIcon, ClockIcon } from "lucide-react";
import { format, startOfDay } from "date-fns";
import { Calendar } from "@/components/ui/calendar";
import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";

// Keep the date and time in the browser's timezone. AdminDialog converts the
// complete local datetime to RFC3339 only when submitting to the Go API.
export function ScreeningTimePicker({ disabled }: { disabled: boolean }) {
  const id = useId();
  const trigger = useRef<HTMLButtonElement>(null);
  const [date, setDate] = useState<Date>();
  const [time, setTime] = useState("19:00");
  const [open, setOpen] = useState(false);
  const localDateTime =
    date && time ? `${format(date, "yyyy-MM-dd")}T${time}` : "";

  return (
    <div className="screening-time-picker">
      <p className="screening-time-label" id={`${id}-label`}>
        Screening time (your local timezone)
      </p>
      <div className="screening-time-controls">
        <Button
          ref={trigger}
          type="button"
          variant="outline"
          className="h-11 w-full justify-between font-normal"
          disabled={disabled}
          aria-labelledby={`${id}-label ${id}-date`}
          aria-expanded={open}
          aria-controls={`${id}-calendar`}
          onClick={() => setOpen((current) => !current)}
        >
          <span id={`${id}-date`}>
            {date ? format(date, "PPP") : "Select a date"}
          </span>
          <CalendarIcon aria-hidden="true" />
        </Button>
        <div className="screening-time-input">
          <ClockIcon aria-hidden="true" size={16} />
          <Input
            type="time"
            aria-label="Screening time of day"
            value={time}
            onChange={(event) => setTime(event.target.value)}
            disabled={disabled}
            required
            step={60}
            className="h-11 pl-9"
          />
        </div>
      </div>
      {open && (
        <div
          id={`${id}-calendar`}
          className="screening-calendar-panel"
          onKeyDown={(event) => {
            if (event.key === "Escape") {
              event.preventDefault();
              event.stopPropagation();
              setOpen(false);
              trigger.current?.focus();
            }
          }}
        >
          <Calendar
            mode="single"
            selected={date}
            defaultMonth={date}
            onSelect={(selected) => {
              setDate(selected);
              if (selected) {
                setOpen(false);
                trigger.current?.focus();
              }
            }}
            disabled={(day) => disabled || day < startOfDay(new Date())}
            autoFocus
          />
        </div>
      )}
      <input type="hidden" name="starts_at" value={localDateTime} />
      <p className="fine muted">
        Pick a future date and time. Times use your device’s timezone.
      </p>
    </div>
  );
}
