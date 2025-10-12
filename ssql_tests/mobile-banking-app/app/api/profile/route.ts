import { type NextRequest, NextResponse } from "next/server";
import { executeQuery, executeWriteQuery } from "@/lib/wwfsdb";
import { getUserIdFromRequest } from "@/lib/auth";

export async function GET(request: NextRequest) {
  try {
    const userId = getUserIdFromRequest(request);
    if (!userId) {
      return NextResponse.json(
        { success: false, message: "Unauthorized" },
        { status: 401 }
      );
    }

    const query = `SELECT user_id, username, email, full_name, phone_number, address, created_at, updated_at FROM users WHERE user_id = '${userId}' LIMIT 1`;
    const users = await executeQuery<any[]>(query);

    if (!users || users.length === 0) {
      return NextResponse.json(
        { success: false, message: "User not found" },
        { status: 404 }
      );
    }

    return NextResponse.json({ success: true, profile: users[0] });

  } catch (error) {
    console.error("[PROFILE_GET_ERROR]", error);
    return NextResponse.json(
      { success: false, message: "Server error" },
      { status: 500 }
    );
  }
}

export async function PUT(request: NextRequest) {
    try {
        const userId = getUserIdFromRequest(request);
        if (!userId) {
          return NextResponse.json(
            { success: false, message: "Unauthorized" },
            { status: 401 }
          );
        }

        const { full_name, email, phone_number, address } = await request.json();
        const updated_at = new Date().toISOString();

        const updateQuery = `
            UPDATE users 
            SET full_name = '${full_name}', email = '${email}', phone_number = '${phone_number}', address = '${address}', updated_at = '${updated_at}'
            WHERE user_id = '${userId}';
        `;

        await executeWriteQuery(updateQuery);

        const updatedProfile = {
            user_id: userId,
            full_name,
            email,
            phone_number,
            address,
            updated_at
        }

        return NextResponse.json({
            success: true,
            profile: updatedProfile,
            message: "Profile updated successfully",
        });

    } catch (error) {
        console.error("[PROFILE_PUT_ERROR]", error);
        return NextResponse.json(
            { success: false, message: "Server error" },
            { status: 500 }
        );
    }
}
