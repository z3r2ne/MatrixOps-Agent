/**
 * HTTP 客户端
 * 提供统一的请求接口、错误处理和通知功能
 */

export interface RequestConfig extends RequestInit {
  params?: Record<string, string | number | boolean>;
  skipErrorNotification?: boolean; // 是否跳过自动错误通知
}

export interface ApiError {
  message: string;
  status: number;
  data?: any;
}

export function getApiErrorMessage(error: unknown, fallback = "请求失败"): string {
  if (error && typeof error === "object" && "message" in error) {
    const message = (error as ApiError).message;
    if (typeof message === "string" && message.trim()) {
      return message;
    }
  }
  if (error instanceof Error && error.message.trim()) {
    return error.message;
  }
  return fallback;
}

// 通知回调函数类型
export type NotifyCallback = (payload: {
  type: 'success' | 'error' | 'info';
  title: string;
  description?: string;
}) => void;

class HttpClient {
  private baseURL: string;
  private notifyCallback: NotifyCallback | null = null;

  constructor(baseURL: string) {
    this.baseURL = baseURL;
  }

  /**
   * 设置通知回调函数
   * 在应用初始化时由 NotificationProvider 调用
   */
  setNotifyCallback(callback: NotifyCallback | null) {
    this.notifyCallback = callback;
  }

  /**
   * 构建完整的 URL
   */
  private buildURL(endpoint: string, params?: Record<string, string | number | boolean>): string {
    // 拼接 baseURL 和 endpoint
    let url = `${this.baseURL}${endpoint}`;
    
    // 处理查询参数
    if (params && Object.keys(params).length > 0) {
      const searchParams = new URLSearchParams();
      Object.entries(params).forEach(([key, value]) => {
        searchParams.append(key, String(value));
      });
      url += `?${searchParams.toString()}`;
    }
    
    return url;
  }

  /**
   * 处理响应
   */
  private async handleResponse<T>(response: Response): Promise<T> {
    // 处理 204 No Content
    if (response.status === 204) {
      return {} as T;
    }

    // 尝试解析 JSON
    let data: any;
    try {
      data = await response.json();
    } catch {
      // 如果不是 JSON 响应，返回空对象
      data = {};
    }

    // 处理错误响应
    if (!response.ok) {
      const error: ApiError = {
        message: data.error || data.message || `请求失败 (HTTP ${response.status})`,
        status: response.status,
        data,
      };
      throw error;
    }

    return data;
  }

  /**
   * 处理错误
   */
  private handleError(error: any, skipNotification: boolean = false): never {
    let apiError: ApiError;

    if (error.status && error.message) {
      // 已经是 ApiError 格式
      apiError = error;
    } else if (error instanceof Error) {
      // 网络错误或其他异常
      apiError = {
        message: error.message || '网络请求失败',
        status: 0,
      };
    } else {
      // 未知错误
      apiError = {
        message: '未知错误',
        status: 0,
      };
    }

    // 发送错误通知
    if (!skipNotification && this.notifyCallback) {
      this.notifyCallback({
        type: 'error',
        title: '请求失败',
        description: apiError.message,
      });
    }

    throw apiError;
  }

  /**
   * 通用请求方法
   */
  async request<T>(endpoint: string, config: RequestConfig = {}): Promise<T> {
    const { params, skipErrorNotification = false, ...fetchOptions } = config;

    try {
      const url = this.buildURL(endpoint, params);
      
      const response = await fetch(url, {
        ...fetchOptions,
        headers: {
          'Content-Type': 'application/json',
          ...fetchOptions.headers,
        },
      });

      return await this.handleResponse<T>(response);
    } catch (error) {
      return this.handleError(error, skipErrorNotification);
    }
  }

  /**
   * GET 请求
   */
  async get<T>(endpoint: string, config?: RequestConfig): Promise<T> {
    return this.request<T>(endpoint, {
      ...config,
      method: 'GET',
    });
  }

  /**
   * POST 请求
   */
  async post<T>(endpoint: string, data?: any, config?: RequestConfig): Promise<T> {
    return this.request<T>(endpoint, {
      ...config,
      method: 'POST',
      body: data ? JSON.stringify(data) : undefined,
    });
  }

  /**
   * PUT 请求
   */
  async put<T>(endpoint: string, data?: any, config?: RequestConfig): Promise<T> {
    return this.request<T>(endpoint, {
      ...config,
      method: 'PUT',
      body: data ? JSON.stringify(data) : undefined,
    });
  }

  /**
   * PATCH 请求
   */
  async patch<T>(endpoint: string, data?: any, config?: RequestConfig): Promise<T> {
    return this.request<T>(endpoint, {
      ...config,
      method: 'PATCH',
      body: data ? JSON.stringify(data) : undefined,
    });
  }

  /**
   * DELETE 请求
   */
  async delete<T>(endpoint: string, config?: RequestConfig): Promise<T> {
    return this.request<T>(endpoint, {
      ...config,
      method: 'DELETE',
    });
  }
}

// 导出 HTTP 客户端实例
export const API_BASE_URL = import.meta.env.VITE_API_URL || '/api';
export const httpClient = new HttpClient(API_BASE_URL);
