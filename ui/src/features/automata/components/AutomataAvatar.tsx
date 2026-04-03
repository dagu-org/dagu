import React from 'react';
import { Bot } from 'lucide-react';

import { cn } from '@/lib/utils';

type AutomataAvatarProps = {
  name: string;
  nickname?: string | null;
  iconUrl?: string | null;
  className?: string;
  iconClassName?: string;
};

export function AutomataAvatar({
  name,
  nickname,
  iconUrl,
  className,
  iconClassName,
}: AutomataAvatarProps): React.ReactElement {
  const src = iconUrl?.trim() || '';
  const [imageFailed, setImageFailed] = React.useState(false);

  React.useEffect(() => {
    setImageFailed(false);
  }, [src]);

  const label = nickname?.trim() || name;

  return (
    <div
      className={cn(
        'flex shrink-0 items-center justify-center overflow-hidden rounded-xl border bg-muted/40 text-muted-foreground',
        className
      )}
      title={label}
    >
      {src && !imageFailed ? (
        <img
          src={src}
          alt=""
          loading="lazy"
          referrerPolicy="no-referrer"
          className="h-full w-full object-cover"
          onError={() => setImageFailed(true)}
        />
      ) : (
        <Bot
          className={cn('h-1/2 w-1/2 text-muted-foreground/80', iconClassName)}
          aria-hidden="true"
        />
      )}
    </div>
  );
}
