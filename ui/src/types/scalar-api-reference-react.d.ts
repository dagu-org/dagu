declare module '@scalar/api-reference-react' {
  import type * as React from 'react';

  export const ApiReferenceReact: React.ComponentType<{
    configuration: Record<string, unknown>;
  }>;

  const DefaultExport: typeof ApiReferenceReact;
  export default DefaultExport;
}

declare module '@scalar/api-reference-react/style.css';
