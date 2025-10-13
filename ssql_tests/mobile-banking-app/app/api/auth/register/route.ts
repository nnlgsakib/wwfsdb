import { type NextRequest, NextResponse } from "next/server";
import { executeQuery, executeWriteQuery } from "@/lib/wwfsdb";
import { v4 as uuidv4 } from "uuid";
import bcrypt from "bcryptjs";
import jwt from "jsonwebtoken";

const JWT_SECRET = process.env.JWT_SECRET || "your-super-secret-key";

export async function POST(request: NextRequest) {
  try {
    const { email, password, full_name, phone_number } = await request.json();

    if (!email || !password || !full_name) {
      return NextResponse.json(
        { success: false, message: "Missing required fields" },
        { status: 400 }
      );
    }

    // Check if user already exists
    const existingUserQuery = `SELECT user_id FROM users WHERE email = '${email}'`;
    const existingUsers = await executeQuery<any[]>(existingUserQuery);

    if (existingUsers && existingUsers.length > 0) {
      return NextResponse.json(
        { success: false, message: "User with this email already exists" },
        { status: 409 }
      );
    }

    // Hash password
    const password_hash = await bcrypt.hash(password, 10);

    const newUser = {
      user_id: uuidv4(),
      username: email.split("@")[0] + Math.floor(Math.random() * 1000), // Add random numbers to make it more unique
      email,
      password_hash,
      full_name,
      phone_number: phone_number || "",
      address: "",
      created_at: new Date().toISOString(),
      updated_at: new Date().toISOString(),
    };

    // Begin transaction using multiple queries in sequence
    const initialAccountId = uuidv4();
    const initialAccountNumber = Math.floor(Math.random() * 9000000000) + 1000000000; // 10-digit random account number
    const currency = "USD"; // Default currency
    
    // Create user and account queries in sequence to maintain consistency
    const queries = [
      `INSERT INTO users (user_id, username, password_hash, email, full_name, phone_number, address, created_at, updated_at) 
        VALUES ('${newUser.user_id}', '${newUser.username}', '${newUser.password_hash}', '${newUser.email}', '${newUser.full_name}', '${newUser.phone_number}', '${newUser.address}', '${newUser.created_at}', '${newUser.updated_at}');`,
      `INSERT INTO accounts (account_id, user_id, account_number, account_type, balance, currency, status, created_at) 
        VALUES ('${initialAccountId}', '${newUser.user_id}', '${initialAccountNumber}', 'Checking', 0.00, '${currency}', 'Active', '${newUser.created_at}');`
    ];
    
    // Execute queries in sequence within the same session
    for (const query of queries) {
      await executeWriteQuery(query);
    }
    
    // Don't return password hash to client
    const userForToken = { ...newUser };
    delete (userForToken as any).password_hash;


    // Generate JWT
    const token = jwt.sign({ userId: newUser.user_id }, JWT_SECRET, {
      expiresIn: "7d",
    });

    return NextResponse.json({
      success: true,
      user: userForToken,
      token,
    });
  } catch (error) {
    console.error("[REGISTER_ERROR]", error);
    return NextResponse.json(
      { success: false, message: "Server error" },
      { status: 500 }
    );
  }
}
