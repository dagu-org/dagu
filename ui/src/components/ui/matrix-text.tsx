import React, { useEffect, useRef, useState } from 'react';

import { cn } from '@/lib/utils';

type MatrixTextProps = {
  text: string;
  className?: string;
  glowSpeed?: number;
};

function MatrixText({ text, className, glowSpeed = 175 }: MatrixTextProps) {
  const [glowPosition, setGlowPosition] = useState(0);
  const intervalRef = useRef<NodeJS.Timeout | null>(null);
  const prefersReducedMotion = useRef(false);
  const textChars = text.split('');

  useEffect(() => {
    const mediaQuery = window.matchMedia('(prefers-reduced-motion: reduce)');
    prefersReducedMotion.current = mediaQuery.matches;

    const handler = (event: MediaQueryListEvent) => {
      prefersReducedMotion.current = event.matches;
    };
    mediaQuery.addEventListener('change', handler);
    return () => mediaQuery.removeEventListener('change', handler);
  }, []);

  useEffect(() => {
    if (prefersReducedMotion.current || textChars.length === 0) {
      return;
    }

    intervalRef.current = setInterval(() => {
      setGlowPosition((prev) => (prev + 1) % textChars.length);
    }, glowSpeed);

    return () => {
      if (intervalRef.current) {
        clearInterval(intervalRef.current);
      }
    };
  }, [glowSpeed, textChars.length]);

  return (
    <span
      className={cn('inline-flex', className)}
      aria-label={text}
      role="status"
    >
      <span className="sr-only">{text}</span>
      <span aria-hidden="true">
        {textChars.map((char, index) => (
          <span
            key={index}
            className="inline-block transition-colors duration-200"
            style={index === glowPosition ? { color: '#86efac' } : undefined}
          >
            {char}
          </span>
        ))}
      </span>
    </span>
  );
}

export type { MatrixTextProps };
export { MatrixText };
export default MatrixText;
