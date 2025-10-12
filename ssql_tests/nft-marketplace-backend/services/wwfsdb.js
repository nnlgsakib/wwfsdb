const axios = require('axios');

const RPC_SERVER_URL = 'http://localhost:8080/rpc';
const DB_NAME = 'newdba';
const PRIVATE_KEY = '3e1a02c509bbdf555ce18daa5f6eaaec6504e7e5a58706c2185eaf910e8872a7';

async function executeQuery(query) {
  try {
    const response = await axios.post(RPC_SERVER_URL, {
      method: 'wwfs.ExecuteQuery',
      params: [{
        db_name: DB_NAME,
        query: query,
      }],
      id: 1,
    });

    if (response.data.error) {
      throw new Error(response.data.error.message);
    }

    // The result from the wwfsdb is a stringified JSON.
    // We need to parse it.
    return JSON.parse(response.data.result.result);
  } catch (error) {
    if (error.response) {
      // The request was made and the server responded with a status code
      // that falls out of the range of 2xx
      console.error('Error response data:', error.response.data);
      console.error('Error response status:', error.response.status);
      console.error('Error response headers:', error.response.headers);
    } else if (error.request) {
      // The request was made but no response was received
      console.error('Error request:', error.request);
    } else {
      // Something happened in setting up the request that triggered an Error
      console.error('Error message:', error.message);
    }
    console.error('Error config:', error.config);
    throw error;
  }
}

async function executeWriteQuery(query) {
    try {
      const response = await axios.post(RPC_SERVER_URL, {
        method: 'wwfs.ExecuteQuery',
        params: [{
          db_name: DB_NAME,
          query: query,
          private_key: PRIVATE_KEY,
        }],
        id: 1,
      });
  
      if (response.data.error) {
        throw new Error(response.data.error.message);
      }
  
      return response.data.result;
    } catch (error) {
        if (error.response) {
            console.error('Error response data:', error.response.data);
            console.error('Error response status:', error.response.status);
            console.error('Error response headers:', error.response.headers);
        } else if (error.request) {
            console.error('Error request:', error.request);
        } else {
            console.error('Error message:', error.message);
        }
        console.error('Error config:', error.config);
        throw error;
    }
  }

module.exports = {
  executeQuery,
  executeWriteQuery,
};
