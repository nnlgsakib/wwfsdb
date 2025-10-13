import { type NextRequest, NextResponse } from "next/server";
import {
  executeQuery,
  executeWriteQuery,
} from "@/lib/wwfsdb";
import { getUserIdFromRequest } from "@/lib/auth";
import { v4 as uuidv4 } from "uuid";

export async function POST(request: NextRequest) {
  const userId = getUserIdFromRequest(request);
  if (!userId) {
    return NextResponse.json(
      { success: false, message: "Unauthorized" },
      { status: 401 }
    );
  }

  try {
    const { from_account_id, to_account_number, amount, description } = await request.json();
    const transferAmount = parseFloat(amount);

    if (!from_account_id || !to_account_number || isNaN(transferAmount) || transferAmount <= 0) {
      return NextResponse.json(
        { success: false, message: "Invalid transfer details" },
        { status: 400 }
      );
    }

    // 1. Verify sender's account and balance
    const senderAccountQuery = `SELECT * FROM accounts WHERE account_id = '${from_account_id}' AND user_id = '${userId}'`;
    const senderAccounts = await executeQuery<any[]>(senderAccountQuery);

    if (!senderAccounts || senderAccounts.length === 0) {
      return NextResponse.json(
        { success: false, message: "Sender account not found or does not belong to user" },
        { status: 403 }
      );
    }

    const senderAccount = senderAccounts[0];
    if (senderAccount.balance < transferAmount) {
      return NextResponse.json(
        { success: false, message: "Insufficient funds" },
        { status: 400 }
      );
    }

    // 2. Find receiver's account
    const receiverAccountQuery = `SELECT account_id FROM accounts WHERE account_number = '${to_account_number}'`;
    const receiverAccounts = await executeQuery<any[]>(receiverAccountQuery);

    if (!receiverAccounts || receiverAccounts.length === 0) {
      return NextResponse.json(
        { success: false, message: "Recipient account not found" },
        { status: 404 }
      );
    }
    const to_account_id = receiverAccounts[0].account_id;

    if (from_account_id === to_account_id) {
        return NextResponse.json(
            { success: false, message: "Cannot transfer to the same account" },
            { status: 400 }
        );
    }

    // 3. Perform transaction
    const transaction_id = uuidv4();
    const created_at = new Date().toISOString();

    // Execute queries individually instead of as a transaction block
    // This is less atomic but more compatible with WWFSDB
    try {
      // 1. Deduct from sender's account
      const deductQuery = `UPDATE accounts SET balance = balance - ${transferAmount} WHERE account_id = '${from_account_id}';`;
      await executeWriteQuery(deductQuery);
      
      // 2. Add to receiver's account
      const addQuery = `UPDATE accounts SET balance = balance + ${transferAmount} WHERE account_id = '${to_account_id}';`;
      await executeWriteQuery(addQuery);
      
      // 3. Insert transaction record
      const insertQuery = `INSERT INTO transactions (transaction_id, from_account_id, to_account_id, amount, currency, transaction_type, description, status, created_at) VALUES ('${transaction_id}', '${from_account_id}', '${to_account_id}', ${transferAmount}, 'USD', 'Transfer', '${description || "Money Transfer"}', 'Completed', '${created_at}');`;
      await executeWriteQuery(insertQuery);
    } catch (queryError) {
      console.error("[TRANSFER_QUERIES_ERROR]", queryError);
      // Try to rollback by reversing the operations
      try {
        // Reverse the deduction
        await executeWriteQuery(`UPDATE accounts SET balance = balance + ${transferAmount} WHERE account_id = '${from_account_id}';`);
        // Reverse the addition
        await executeWriteQuery(`UPDATE accounts SET balance = balance - ${transferAmount} WHERE account_id = '${to_account_id}';`);
      } catch (rollbackError) {
        console.error("[TRANSFER_ROLLBACK_ERROR]", rollbackError);
      }
      throw queryError;
    }

    const transaction = {
        transaction_id,
        from_account_id,
        to_account_id,
        amount: transferAmount,
        currency: "USD",
        transaction_type: "Transfer",
        description: description || "Money Transfer",
        status: "Completed",
        created_at,
      }

    return NextResponse.json({
      success: true,
      transaction,
      message: "Transfer completed successfully",
    });

  } catch (error) {
    console.error("[TRANSFER_POST_ERROR]", error);
    return NextResponse.json(
      { success: false, message: "Server error during transfer" },
      { status: 500 }
    );
  }
}
