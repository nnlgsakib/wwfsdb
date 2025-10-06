const http = require('http');
const readline = require('readline');

const dbName = 'cli_app_db';

async function sendQuery(dbName, query) {
  return new Promise((resolve, reject) => {
    const postData = JSON.stringify({
      db_name: dbName,
      query: query,
    });

    const options = {
      hostname: 'localhost',
      port: 8080,
      path: '/query',
      method: 'POST',
      headers: {
        'Content-Type': 'application/json',
        'Content-Length': Buffer.byteLength(postData),
      },
    };

    const req = http.request(options, (res) => {
      let data = '';
      res.on('data', (chunk) => {
        data += chunk;
      });
      res.on('end', () => {
        resolve(JSON.parse(data));
      });
    });

    req.on('error', (e) => {
      reject(e);
    });

    req.write(postData);
    req.end();
  });
}

async function setup() {
  console.log('Setting up the database...');
  await sendQuery('', `CREATE DATABASE ${dbName}`);
  
  const schema = `
    CREATE TABLE employees (
        id INT,
        first_name STRING,
        last_name STRING,
        email STRING,
        phone_number STRING,
        hire_date STRING,
        job_id STRING,
        salary FLOAT,
        commission_pct FLOAT,
        manager_id INT,
        department_id INT
    );
  `;
  await sendQuery(dbName, schema);
  console.log('Database setup complete.');
}

function showMenu() {
  console.log('\nEmployee Management System');
  console.log('1. Add Employee');
  console.log('2. List Employees');
  console.log('3. Search Employee by Name');
  console.log('4. Delete Employee by ID');
  console.log('5. Update Employee Salary');
  console.log('6. Calculate Average Salary for Department');
  console.log('7. Exit');
}

async function menuLoop(rl) {
  showMenu();
  const choice = await new Promise(resolve => rl.question('Enter your choice: ', resolve));

  switch (choice) {
      case '1':
        const id = await new Promise(resolve => rl.question('Enter employee ID: ', resolve));
        const firstName = await new Promise(resolve => rl.question('Enter first name: ', resolve));
        const lastName = await new Promise(resolve => rl.question('Enter last name: ', resolve));
        const email = await new Promise(resolve => rl.question('Enter email: ', resolve));
        
        let salary;
        while (true) {
          const salaryInput = await new Promise(resolve => rl.question('Enter salary: ', resolve));
          salary = parseFloat(salaryInput);
          if (!isNaN(salary)) {
            break;
          }
          console.log('Invalid salary. Please enter a number.');
        }

        const deptId = await new Promise(resolve => rl.question('Enter department ID: ', resolve));

        const insertQuery = `INSERT INTO employees VALUES ('${id}', '${firstName}', '${lastName}', '${email}', '', '', '', '${salary}', '', '', '${deptId}');`;
        const insertRes = await sendQuery(dbName, insertQuery);
        console.log(insertRes);
        break;
    case '2':
      const selectRes = await sendQuery(dbName, 'SELECT * FROM employees');
      if (selectRes.result) {
        try {
          const employees = JSON.parse(selectRes.result);
          console.table(employees);
        } catch (e) {
          console.log(selectRes.result);
        }
      } else {
        console.log(selectRes.error);
      }
      break;
    case '3':
      const name = await new Promise(resolve => rl.question('Enter employee name to search: ', resolve));
      const searchRes = await sendQuery(dbName, `SELECT * FROM employees WHERE first_name = '${name}'`);
      if (searchRes.result) {
        try {
          const employees = JSON.parse(searchRes.result);
          console.table(employees);
        } catch (e) {
          console.log(searchRes.result);
        }
      } else {
        console.log(searchRes.error);
      }
      break;
    case '4':
      const deleteId = await new Promise(resolve => rl.question('Enter employee ID to delete: ', resolve));
      const deleteRes = await sendQuery(dbName, `DELETE FROM employees WHERE id = '${deleteId}'`);
      console.log(deleteRes);
      break;
    case '5':
      const updateId = await new Promise(resolve => rl.question('Enter employee ID to update: ', resolve));
      const newSalary = await new Promise(resolve => rl.question('Enter new salary: ', resolve));
      const updateRes = await sendQuery(dbName, `UPDATE employees SET salary = '${newSalary}' WHERE id = '${updateId}'`);
      console.log(updateRes);
      break;
    case '6':
      const deptIdForAvg = await new Promise(resolve => rl.question('Enter department ID to calculate average salary: ', resolve));
      const deptRes = await sendQuery(dbName, `SELECT * FROM employees WHERE department_id = '${deptIdForAvg}'`);
      if (deptRes.result) {
        try {
          const employees = JSON.parse(deptRes.result);
          if (employees.length > 0) {
            const totalSalary = employees.reduce((acc, emp) => acc + parseFloat(emp.salary), 0);
            const avgSalary = totalSalary / employees.length;
            console.log(`Average salary for department ${deptIdForAvg}: ${avgSalary}`);
          } else {
            console.log(`No employees found in department ${deptIdForAvg}`);
          }
        } catch (e) {
          console.log(deptRes.result);
        }
      } else {
        console.log(deptRes.error);
      }
      break;
    case '7':
      rl.close();
      return;
    default:
      console.log('Invalid choice.');
  }

  menuLoop(rl);
}

async function main() {
  await setup();

  const rl = readline.createInterface({
    input: process.stdin,
    output: process.stdout
  });

  rl.on('close', () => {
    console.log('Readline interface closed.');
    process.exit(0);
  });

  menuLoop(rl);
}

main();