import {get, post} from "@/plugin/utils/request";

export const login = (params) => post('/api/v1/user/login', params)
export const createUser = (params) => post('/api/v1/user/register', params)
export const changePassword = (params) => post('/api/v1/user/password', params)
export const fetchUserPrincipals = (params) => get('/api/v1/k8s/service-tree/principals', {
  principalType: 'user',
  ...params,
})
export const fetchRolePrincipals = (params) => get('/api/v1/k8s/service-tree/principals', {
  principalType: 'role',
  ...params,
})
