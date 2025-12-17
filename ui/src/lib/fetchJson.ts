/**
 * Fetches JSON from the configured API base URL and returns the parsed response body.
 *
 * This function appends `input` to `getConfig().apiURL`, merges any provided `init.headers`
 * with `Accept: application/json`, and, if present, adds an `Authorization: Bearer <token>`
 * header using the token stored in localStorage under `dagu_auth_token`.
 *
 * @param input - The request URL or RequestInfo which will be appended to the API base URL
 * @param init - Optional fetch init options; provided headers are merged with the function's headers
 * @returns The parsed response body as JSON
 * @throws FetchError when the response has a non-OK status; the error includes the original `Response` and the parsed response body
 */
export default async function fetchJson<JSON = unknown>(
  input: RequestInfo,
  init?: RequestInit
): Promise<JSON> {
  const headers: HeadersInit = {
    ...(init?.headers || {}),
    Accept: 'application/json',
  };

  const token = localStorage.getItem('dagu_auth_token');
  if (token) {
    (headers as Record<string, string>)['Authorization'] = `Bearer ${token}`;
  }

  const response = await fetch(`${getConfig().apiURL}${input}`, {
    ...init,
    headers,
  });
  const data = await response.json();

  if (response.ok) {
    return data;
  }

  throw new FetchError({
    message: response.statusText,
    response,
    data,
  });
}

export class FetchError extends Error {
  response: Response;
  data: {
    message: string;
  };
  constructor({
    message,
    response,
    data,
  }: {
    message: string;
    response: Response;
    data: {
      message: string;
    };
  }) {
    super(message);

    if (Error.captureStackTrace) {
      Error.captureStackTrace(this, FetchError);
    }

    this.name = 'FetchError';
    this.response = response;
    this.data = data ?? { message: message };
  }
}