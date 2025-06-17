// Plugin to convert issue numbers to GitHub links
export function issueLinksPlugin(md) {
  // Save the original text renderer
  const defaultTextRenderer = md.renderer.rules.text;

  // Override the text renderer
  md.renderer.rules.text = function(tokens, idx, options, env, renderer) {
    const token = tokens[idx];
    
    // Skip if we're inside a code block or inline code
    if (env.inCode || token.type !== 'text') {
      return defaultTextRenderer ? defaultTextRenderer(tokens, idx, options, env, renderer) : token.content;
    }
    
    // Replace issue patterns
    let content = token.content;
    
    // Match patterns like (#123) or #123 followed by comma, space, or end of string
    content = content.replace(
      /\(#(\d+)\)|#(\d+)(?=[\s,)]|$)/g,
      (match, num1, num2) => {
        const issueNum = num1 || num2;
        const prefix = match.startsWith('(') ? '(' : '';
        const suffix = match.endsWith(')') ? ')' : '';
        return `${prefix}<a href="https://github.com/dagu-org/dagu/issues/${issueNum}" target="_blank" rel="noopener">#${issueNum}</a>${suffix}`;
      }
    );
    
    return content;
  };
}