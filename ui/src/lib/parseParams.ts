export type Parameter = {
  Name?: string;
  Value: string;
};

function unescapeValue(s: string): string {
  let result = '';
  for (let i = 0; i < s.length; i++) {
    if (s[i] === '\\' && i + 1 < s.length) {
      const next = s[i + 1];
      switch (next) {
        case 'n':
          result += '\n';
          i++;
          break;
        case 't':
          result += '\t';
          i++;
          break;
        case '\\':
          result += '\\';
          i++;
          break;
        case '"':
          result += '"';
          i++;
          break;
        default:
          result += s[i];
          break;
      }
    } else {
      result += s[i];
    }
  }
  return result;
}

export function parseParams(input: string): Parameter[] {
  const paramRegex = /(?:([^\s=]+)=)?("(?:\\"|[^"])*"|[^"\s]+)/g;
  const params: Parameter[] = [];

  let match;
  while ((match = paramRegex.exec(input)) !== null) {
    const [, name, value] = match;

    const param: Parameter = {
      Value: value?.startsWith('"')
        ? unescapeValue(value.slice(1, -1))
        : value || '',
    };

    if (name) {
      param.Name = name;
    }

    params.push(param);
  }

  return params;
}

export function stringifyParams(params: Parameter[]): string {
  if (params.length === 0) {
    return '';
  }
  const items = params.map((p) => {
    // Normalize CR+LF and standalone CR to LF
    const value = p.Value.replace(/\r\n/g, '\n').replace(/\r/g, '\n');
    if (p.Name) {
      return { [p.Name]: value };
    }
    return value;
  });
  return JSON.stringify(items);
}
