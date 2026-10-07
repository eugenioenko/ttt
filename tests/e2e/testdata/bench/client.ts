import type { IncomingHttpHeaders } from 'node:http';

export type Method = 'GET' | 'POST' | 'PUT' | 'DELETE';

export interface RequestOptions<TBody = unknown> {
  method?: Method;
  headers?: Record<string, string>;
  body?: TBody;
  retries?: number;
  signal?: AbortSignal;
}

export interface ApiResponse<T> {
  status: number;
  headers: IncomingHttpHeaders;
  data: T;
}

export class HttpError extends Error {
  constructor(
    public readonly status: number,
    message: string,
    public readonly body?: unknown,
  ) {
    super(`HTTP ${status}: ${message}`);
  }
}

type Interceptor = <T>(response: ApiResponse<T>) => ApiResponse<T> | Promise<ApiResponse<T>>;

const sleep = (ms: number): Promise<void> => new Promise((resolve) => setTimeout(resolve, ms));

export class ApiClient {
  private readonly interceptors: Interceptor[] = [];

  constructor(private readonly baseUrl: string, private readonly token?: string) {}

  intercept(interceptor: Interceptor): this {
    this.interceptors.push(interceptor);
    return this;
  }

  async request<TResult, TBody = unknown>(
    path: string,
    { method = 'GET', headers = {}, body, retries = 2, signal }: RequestOptions<TBody> = {},
  ): Promise<ApiResponse<TResult>> {
    const url = new URL(path, this.baseUrl);
    for (let attempt = 0; ; attempt++) {
      const res = await fetch(url, {
        method,
        signal,
        headers: {
          'content-type': 'application/json',
          ...(this.token ? { authorization: `Bearer ${this.token}` } : {}),
          ...headers,
        },
        body: body === undefined ? undefined : JSON.stringify(body),
      });
      if (res.status >= 500 && attempt < retries) {
        await sleep(2 ** attempt * 100);
        continue;
      }
      if (!res.ok) throw new HttpError(res.status, res.statusText, await res.text());
      let response: ApiResponse<TResult> = {
        status: res.status,
        headers: Object.fromEntries(res.headers) as IncomingHttpHeaders,
        data: (await res.json()) as TResult,
      };
      for (const interceptor of this.interceptors) response = await interceptor(response);
      return response;
    }
  }
}
