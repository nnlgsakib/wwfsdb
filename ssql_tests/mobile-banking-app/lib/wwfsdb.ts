
import axios from "axios";

const RPC_SERVER_URL = "http://localhost:8080/rpc";
const DB_NAME = "mbs_prod";
const PRIVATE_KEY = "a20be6eeca6fb5f86c6a5616c28fa45da4b53a2ae616f8adff58de1fc4510d2e"; 

interface RpcResponse {
    result?: {
        result: string;
        session_id?: string;
    };
    error?: any;
}

async function executeQuery<T>(query: string): Promise<T> {
  try {
    const response = await axios.post<RpcResponse>(RPC_SERVER_URL, {
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

    if (response.data.result) {
        // The result from the wwfsdb is a stringified JSON.
        // We need to parse it.
        return JSON.parse(response.data.result.result) as T;
    }
    
    throw new Error("Invalid RPC response format");

  } catch (error) {
    if (axios.isAxiosError(error)) {
        if (error.response) {
            console.error('Error response data:', error.response.data);
            console.error('Error response status:', error.response.status);
        } else if (error.request) {
            console.error('Error request:', error.request);
        } else {
            console.error('Error message:', error.message);
        }
    } else {
        console.error('Unexpected error:', error);
    }
    throw error;
  }
}

async function executeWriteQuery(query: string): Promise<any> {
    try {
      const response = await axios.post<RpcResponse>(RPC_SERVER_URL, {
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
        if (axios.isAxiosError(error)) {
            if (error.response) {
                console.error('Error response data:', error.response.data);
                console.error('Error response status:', error.response.status);
            } else if (error.request) {
                console.error('Error request:', error.request);
            } else {
                console.error('Error message:', error.message);
            }
        } else {
            console.error('Unexpected error:', error);
        }
        throw error;
    }
  }

export { executeQuery, executeWriteQuery };
