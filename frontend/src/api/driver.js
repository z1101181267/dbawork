import http from './http'

// 驱动列表（?db_type=）
export const listDrivers = (params) => http.get('/drivers', { params })

// 全量检测
export const detectAll = () => http.post('/drivers/detect', {}, { timeout: 300000 })

// 单类型检测
export const detectOne = (dbType) => http.post(`/drivers/detect/${dbType}`, {}, { timeout: 60000 })

// 安装 python 驱动
export const installDriver = (data) => http.post('/drivers/install', data, { timeout: 600000 })

// 卸载 python 驱动
export const uninstallDriver = (data) => http.post('/drivers/uninstall', data, { timeout: 300000 })

// 上传 JAR（multipart）
export const uploadJar = (formData) =>
  http.post('/drivers/jdbc', formData, {
    headers: { 'Content-Type': 'multipart/form-data' },
    timeout: 300000
  })

// 删除驱动
export const removeDriver = (id) => http.delete(`/drivers/${id}`)

// 设为默认
export const activateDriver = (id) => http.post(`/drivers/${id}/activate`, {})

// 类型目录（数据源表单 / 筛选共用）
export const listDbTypes = () => http.get('/db-types')
