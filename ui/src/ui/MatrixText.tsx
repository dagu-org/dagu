import React, { useEffect, useRef, useState } from 'react';
import { cn } from '@/lib/utils';

type Props = {
  /** The text to display */
  text: string;
  /** Additional CSS classes */
  className?: string;
  /** Speed of glow movement in ms per character (default: 175) */
  glowSpeed?: number;
};

/**
 * MatrixText displays text with a glowing wave effect.
 * Each character glows one by one, moving left→right continuously.
 */
function MatrixText({ text, className, glowSpeed = 175 }: Props) {
  const [glowPosition, setGlowPosition] = useState(0);
  const intervalRef = useRef<NodeJS.Timeout | null>(null);
  const prefersReducedMotion = useRef(false);

  const textChars = text.split('');

  // Check for reduced motion preference
  useEffect(() => {
    const mediaQuery = window.matchMedia('(prefers-reduced-motion: reduce)');
    prefersReducedMotion.current = mediaQuery.matches;

    const handler = (e: MediaQueryListEvent) => {
      prefersReducedMotion.current = e.matches;
    };
    mediaQuery.addEventListener('change', handler);
    return () => mediaQuery.removeEventListener('change', handler);
  }, []);

  // Move glow position left→right continuously
  useEffect(() => {
    if (prefersReducedMotion.current) return;

    intervalRef.current = setInterval(() => {
      setGlowPosition((prev) => (prev + 1) % textChars.length);
    }, glowSpeed);

    return () => {
      if (intervalRef.current) clearInterval(intervalRef.current);
    };
  }, [glowSpeed, textChars.length]);

  return (
    <span
      className={cn('inline-flex', className)}
      aria-label={text}
      role="status"
    >
      {/* Hidden text for screen readers */}
      <span className="sr-only">{text}</span>
      {/* Visible text with glow effect */}
      <span aria-hidden="true">
        {textChars.map((char, index) => {
          const isLeading = index === glowPosition;
          const isTrailing = index === (glowPosition - 1 + textChars.length) % textChars.length;

          let style: React.CSSProperties | undefined;
          if (isLeading) {
            style = { color: '#8bc48b' }; // Subtle bright
          } else if (isTrailing) {
            style = { color: '#7db07d' }; // Subtle dim
          }

          return (
            <span
              key={index}
              className="inline-block transition-colors duration-150"
              style={style}
            >
              {char}
            </span>
          );
        })}
      </span>
    </span>
  );
}

export default MatrixText;
