export type Parameter = {
  Name?: string;
  Value: string;
};

export function parseParams(input: string): Parameter[] {
  const paramRegex = /(?:([^\s=]+)=)?("(?:\\"|[^"])*"|[^"\s]+)/g;
  const params: Parameter[] = [];

  let match;
  while ((match = paramRegex.exec(input)) !== null) {
    const [, name, value] = match;

    const param: Parameter = {
      Value: value.startsWith('"')
        ? value.slice(1, -1).replace(/\\"/g, '"')
        : value,
    };

    if (name) {
      param.Name = name;
    }

    params.push(param);
  }

  return params;
}

export function stringifyParams(params: Parameter[]): string {
  return params
    .map((param) => {
      const escapedValue = param.Value.replace(/"/g, '\\"');
      const quotedValue = `"${escapedValue}"`;

      return param.Name ? `${param.Name}=${quotedValue}` : quotedValue;
    })
    .join(' ');
}
