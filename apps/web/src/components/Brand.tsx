// Pablo mark — a modern, sleek design representing workflow automation.
// Gradient uses a distinctive purple ramp.

export function BrandMark({ size = 20 }: { size?: number }) {
  return (
    <svg
      width={size}
      height={size}
      viewBox="0 0 24 24"
      fill="none"
      xmlns="http://www.w3.org/2000/svg"
      aria-hidden
    >
      <rect width="24" height="24" rx="6" fill="url(#pablo-grad)" />
      <path
        d="M7 10l5-4 5 4"
        stroke="white"
        strokeWidth="1.8"
        strokeLinecap="round"
        strokeLinejoin="round"
      />
      <path
        d="M7 15l5-4 5 4"
        stroke="white"
        strokeWidth="1.8"
        strokeLinecap="round"
        strokeLinejoin="round"
        opacity="0.7"
      />
      <defs>
        <linearGradient id="pablo-grad" x1="0" y1="0" x2="24" y2="24" gradientUnits="userSpaceOnUse">
          <stop offset="0" stopColor="#665efd" />
          <stop offset="0.55" stopColor="#533afd" />
          <stop offset="1" stopColor="#2e2b8c" />
        </linearGradient>
      </defs>
    </svg>
  );
}

// Wordmark — pairs with the symbol. Weight 300 + tight tracking.
export function BrandWordmark({ className = "" }: { className?: string }) {
  return (
    <span
      className={`font-display text-[var(--color-fg)] ${className}`}
      style={{ fontWeight: 300, letterSpacing: "-0.02em" }}
    >
      Pablo
    </span>
  );
}
