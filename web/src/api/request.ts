import axios from 'axios'
import type { AxiosInstance, AxiosResponse } from 'axios'

/** 统一响应结构 */
interface ApiResponse<T = unknown> {
  code: number
  msg: string
  data: T
}

/** 创建axios实例，基础路径为/api/v1 */
const request: AxiosInstance = axios.create({
  baseURL: '/api/v1',
  timeout: 15000,
})

/** 请求拦截器 - 可扩展添加token等 */
request.interceptors.request.use(
  (config) => {
    return config
  },
  (error) => {
    return Promise.reject(error)
  }
)

/** 业务错误类 - 保留错误码和附加数据 */
export class ApiError extends Error {
  code: number
  data: any
  constructor(code: number, msg: string, data?: any) {
    super(msg)
    this.code = code
    this.data = data ?? null
    this.name = 'ApiError'
  }
}

/** 响应拦截器 - 解包{code,msg,data}结构，非零code视为错误 */
request.interceptors.response.use(
  (response: AxiosResponse<ApiResponse>) => {
    const res = response.data
    if (res.code !== 0) {
      return Promise.reject(new ApiError(res.code, res.msg || '请求失败', res.data))
    }
    return res.data as any
  },
  (error) => {
    return Promise.reject(error)
  }
)

export default request
