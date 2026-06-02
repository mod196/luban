import axios from 'axios';

const runtimeClient = axios.create({
    baseURL: process.env.VUE_APP_BASE_URL || '',
    timeout: 3000,
});

export const fetchRuntimeConfig = () => runtimeClient
    .get('/api/v1/luban/runtime-config')
    .then(response => response.data);
