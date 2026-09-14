import http from './http'

export const listGroups = () => http.get('/groups')

export const createGroup = (data) => http.post('/groups', data)

export const updateGroup = (id, data) => http.put(`/groups/${id}`, data)

export const removeGroup = (id) => http.delete(`/groups/${id}`)
