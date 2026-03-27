export function Logo({ className }: { className?: string }) {
  return (
    <svg
      xmlns="http://www.w3.org/2000/svg"
      viewBox="7 6 18 25"
      fill="none"
      className={className}
      role="img"
      aria-label="Sailbox"
    >
      <title>Sailbox</title>
      <polygon points="16,13 23,17 16,21 9,17" fill="currentColor" opacity="0.12" />
      <polygon points="9,17 16,21 16,29 9,25" fill="currentColor" opacity="0.48" />
      <polygon points="23,17 16,21 16,29 23,25" fill="currentColor" opacity="0.22" />
      <polygon points="16,8 23,17 16,13" fill="currentColor" opacity="0.90" />
    </svg>
  );
}
