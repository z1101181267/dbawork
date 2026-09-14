import http from './http'

// 列表（?group_id=&db_type=&status=&keyword=）
export const listDatasources = (params) => http.get('/datasources', { params })

export const getDatasource = (id) => http.get(`/datasources/${id}`)

export const createDatasource = (data) => http.post('/datasources', data, { timeout: 120000 })

export const updateDatasource = (id, data) => http.put(`/datasources/${id}`, data, { timeout: 120000 })

export const removeDatasource = (id) => http.delete(`/datasources/${id}`)

// 测试已保存数据源
export const testDatasource = (id) => http.post(`/datasources/${id}/test`, {}, { timeout: 180000 })

// 未保存前临时测试
export const testAdhoc = (data) => http.post('/datasources/test-adhoc', data, { timeout: 180000 })

// CSV 批量导入（multipart）
export const importCsv = (formData) =>
  http.post('/datasources/import', formData, {
    headers: { 'Content-Type': 'multipart/form-data' },
    timeout: 180000
  })

// 导出（blob 下载）
export const exportCsv = () =>
  http.get('/datasources/export.csv', { responseType: 'blob' })
