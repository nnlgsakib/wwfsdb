import { type NextRequest, NextResponse } from "next/server";
import { executeQuery, executeWriteQuery } from "@/lib/wwfsdb";
import { getUserIdFromRequest } from "@/lib/auth";
import { v4 as uuidv4 } from "uuid";

export async function GET(request: NextRequest) {
  try {
    const userId = getUserIdFromRequest(request);
    if (!userId) {
      return NextResponse.json(
        { success: false, message: "Unauthorized" },
        { status: 401 }
      );
    }

    const query = `SELECT * FROM beneficiaries WHERE user_id = '${userId}' ORDER BY created_at DESC`;
    const beneficiaries = await executeQuery<any[]>(query);

    return NextResponse.json({ success: true, beneficiaries: beneficiaries || [] });

  } catch (error) {
    console.error("[BENEFICIARIES_GET_ERROR]", error);
    return NextResponse.json(
      { success: false, message: "Server error" },
      { status: 500 }
    );
  }
}

export async function POST(request: NextRequest) {
  try {
    const userId = getUserIdFromRequest(request);
    if (!userId) {
      return NextResponse.json(
        { success: false, message: "Unauthorized" },
        { status: 401 }
      );
    }

    const { nickname, account_number, bank_name } = await request.json();

    if (!nickname || !account_number || !bank_name) {
      return NextResponse.json(
        { success: false, message: "Missing required fields" },
        { status: 400 }
      );
    }

    const newBeneficiary = {
      beneficiary_id: uuidv4(),
      user_id: userId,
      nickname,
      account_number,
      bank_name,
      created_at: new Date().toISOString(),
    };

    const insertQuery = `
      INSERT INTO beneficiaries (beneficiary_id, user_id, nickname, account_number, bank_name, created_at)
      VALUES ('${newBeneficiary.beneficiary_id}', '${newBeneficiary.user_id}', '${newBeneficiary.nickname}', '${newBeneficiary.account_number}', '${newBeneficiary.bank_name}', '${newBeneficiary.created_at}');
    `;

    await executeWriteQuery(insertQuery);

    return NextResponse.json({
      success: true,
      beneficiary: newBeneficiary,
      message: "Beneficiary added successfully",
    });

  } catch (error) {
    console.error("[BENEFICIARIES_POST_ERROR]", error);
    return NextResponse.json(
      { success: false, message: "Server error" },
      { status: 500 }
    );
  }
}

export async function DELETE(request: NextRequest) {
  try {
    const userId = getUserIdFromRequest(request);
    if (!userId) {
      return NextResponse.json(
        { success: false, message: "Unauthorized" },
        { status: 401 }
      );
    }

    const { beneficiary_id } = await request.json();

    if (!beneficiary_id) {
        return NextResponse.json(
            { success: false, message: "Beneficiary ID is required" },
            { status: 400 }
        );
    }

    // Optional: Check if the beneficiary belongs to the user before deleting
    const beneficiaries = await executeQuery<any[]>(
      `SELECT user_id FROM beneficiaries WHERE beneficiary_id = '${beneficiary_id}'`
    );

    if (!beneficiaries || beneficiaries.length === 0 || beneficiaries[0].user_id !== userId) {
        return NextResponse.json(
            { success: false, message: "Beneficiary not found or you do not have permission to delete it." },
            { status: 404 }
        );
    }

    const deleteQuery = `DELETE FROM beneficiaries WHERE beneficiary_id = '${beneficiary_id}' AND user_id = '${userId}'`;
    await executeWriteQuery(deleteQuery);

    return NextResponse.json({
      success: true,
      message: "Beneficiary deleted successfully",
    });

  } catch (error) {
    console.error("[BENEFICIARIES_DELETE_ERROR]", error);
    return NextResponse.json(
      { success: false, message: "Server error" },
      { status: 500 }
    );
  }
}
