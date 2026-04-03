import React from 'react';

export function AutomataSwarmIcon({ size = 18 }: { size?: number }): React.ReactElement {
  return (
    <svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 24 24" fill="currentColor" width={size} height={size} shapeRendering="geometricPrecision">
      {/* Stream 1 (bottom — flattens out toward bottom-right) */}
      <circle cx="2.5" cy="22" r=".4"/>
      <circle cx="5" cy="21.5" r=".55"/>
      <circle cx="8" cy="20.5" r=".7"/>
      <circle cx="11.5" cy="19" r=".9"/>
      <circle cx="15" cy="17.5" r="1.1"/>
      <circle cx="19" cy="16" r="1.3"/>
      {/* Stream 2 (middle — straight diagonal) */}
      <circle cx="1.5" cy="20.5" r=".4"/>
      <circle cx="4.5" cy="19" r=".55"/>
      <circle cx="7.5" cy="17" r=".7"/>
      <circle cx="11" cy="14.5" r=".9"/>
      <circle cx="15" cy="12" r="1.1"/>
      <circle cx="19.5" cy="9" r="1.35"/>
      {/* Stream 3 (top — curves up steeply) */}
      <circle cx="2" cy="18" r=".45"/>
      <circle cx="5" cy="15.5" r=".6"/>
      <circle cx="8" cy="13" r=".8"/>
      <circle cx="11.5" cy="10" r="1"/>
      <circle cx="15.5" cy="6.5" r="1.25"/>
      <circle cx="20" cy="3" r="1.5"/>
    </svg>
  );
}
