import { type NextRequest } from "next/server";
import jwt from "jsonwebtoken";

const JWT_SECRET = process.env.JWT_SECRET || "your-super-secret-key";

interface DecodedToken {
  userId: string;
  iat: number;
  exp: number;
}

export function getUserIdFromRequest(request: NextRequest): string | null {
  const authHeader = request.headers.get("Authorization");

  if (!authHeader || !authHeader.startsWith("Bearer ")) {
    return null;
  }

  const token = authHeader.split(" ")[1];

  if (!token) {
    return null;
  }

  try {
    const decoded = jwt.verify(token, JWT_SECRET) as DecodedToken;
    return decoded.userId;
  } catch (error) {
    console.error("[JWT_VERIFY_ERROR]", error);
    return null;
  }
}
