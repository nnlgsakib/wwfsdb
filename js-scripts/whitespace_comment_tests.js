async function runWhitespaceAndCommentTests() {
    console.log('\n--- 🧪 Running Whitespace & Comment Tests ---\n');

    // Create a test table first
    await assertCommandSuccess(`CREATE TABLE comment_test (id INT, name VARCHAR(50), value FLOAT);`, 'Creates a table for comment and whitespace tests');
    
    console.log('  --- Testing Multiline SQL Statements ---');
    // Test multiline SQL with various indentation
    const multilineQuery = `
        INSERT INTO comment_test 
        VALUES 
            (1, 
             'Multiline Test', 
             123.45);`;
    await assertCommandSuccess(multilineQuery, 'Executes multiline SQL with indentation');

    // Test multiline SELECT with indentation
    await assertQueryResult(
        `SELECT 
             id, 
             name, 
             value 
         FROM comment_test 
         WHERE id = 1;`,
        [{"comment_test.id": 1, "comment_test.name": "Multiline Test", "comment_test.value": 123.45}],
        'Executes multiline SELECT with indentation'
    );

    console.log('\n  --- Testing SQL with Comments ---');
    // Test SQL with single-line comments
    await assertCommandSuccess(`INSERT INTO comment_test -- This is a comment
                                VALUES (2, 'Comment Test', 99.99);`, 'Executes SQL with single-line comment');

    // Test SQL with inline comments
    await assertQueryResult(
        `SELECT id /* inline comment */, name FROM comment_test WHERE value > 100; -- This finds values > 100`,
        [{"id": 1, "name": "Multiline Test"}],
        'Executes SELECT with both inline and end-of-line comments'
    );

    // Test multiline comment
    await assertCommandSuccess(`INSERT INTO comment_test VALUES /* multiline
    comment
    spanning
    lines */ (3, 'Multiline Comment Test', 75.25);`, 'Executes SQL with multiline comment');
    
    await assertQueryResult(
        `SELECT name FROM comment_test WHERE id = 3;`,
        [{"name": "Multiline Comment Test"}],
        'Confirms data from multiline comment SQL'
    );

    console.log('\n  --- Testing Various Indentation Styles ---');
    // Test different indentation patterns
    const indentedQueries = [
        `INSERT INTO comment_test VALUES
            (4, 'Tab Indent', 10.0);`,
        `   INSERT INTO comment_test VALUES
              (5, 'Space Indent', 20.0);`,
        `INSERT INTO comment_test
             SELECT 6, 'Continuation Line', 30.0
             ;`
    ];

    for (let i = 0; i < indentedQueries.length; i++) {
        await assertCommandSuccess(indentedQueries[i], `Executes indented query ${i + 1}`);
    }

    // Verify all inserted records
    await assertQueryResult(
        `SELECT COUNT(*) FROM comment_test;`,
        [{"COUNT(*)": 6}],
        'Verifies all records were inserted despite formatting variations'
    );

    console.log('\n  --- Testing Complex Formatting ---');
    // Complex example mixing all features
    await assertCommandSuccess(`
        -- This is a complex test combining various formatting features
        INSERT INTO comment_test 
        SELECT 7, 
               'Complex Format' /* inline comment */,
               42.0    -- end of line comment
        ; -- statement end comment
    `, 'Executes complex formatted SQL');
    
    await assertQueryResult(
        `SELECT name, value FROM comment_test WHERE id = 7; -- with comment`,
        [{"name": "Complex Format", "value": 42.0}],
        'Verifies complex formatted SQL result'
    );
}