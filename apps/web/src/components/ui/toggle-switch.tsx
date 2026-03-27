import { cn } from "@/lib/utils";

interface ToggleSwitchProps {
  checked: boolean;
  onChange: (checked: boolean) => void;
  disabled?: boolean;
  title?: string;
  className?: string;
}

export function ToggleSwitch({ checked, onChange, disabled, title, className }: ToggleSwitchProps) {
  return (
    <button
      type="button"
      onClick={() => !disabled && onChange(!checked)}
      disabled={disabled}
      className={cn(
        "relative inline-flex h-6 w-11 items-center rounded-full border transition-colors",
        checked ? "border-primary bg-primary" : "border-muted-foreground/30 bg-muted-foreground/20",
        disabled && "cursor-not-allowed opacity-50",
        className,
      )}
      title={title}
      aria-checked={checked}
      role="switch"
    >
      <span
        className={cn(
          "inline-block h-4.5 w-4.5 rounded-full bg-background shadow-sm transition-transform",
          checked ? "translate-x-5" : "translate-x-0.5",
        )}
      />
    </button>
  );
}
