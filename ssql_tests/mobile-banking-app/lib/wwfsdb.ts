
import axios from "axios";

const RPC_SERVER_URL = "http://localhost:8080/rpc";
const DB_NAME = "mbs1";
const PRIVATE_KEY = "812300dd3dbe95b946901cd3d5941ada3a4b4f3c7b69d728446626573bbfee99"; 

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
