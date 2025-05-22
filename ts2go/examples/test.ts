/**
 * User interface representing a person
 */
interface User {
  name: string;
  age: number;
  email?: string; // Optional email
}

/**
 * Employee extends User with additional work information
 */
interface Employee extends User {
  department: string;
  title: string;
  salary: number;
}

// A function that works with User objects
function greet(user: User): string {
  return `Hello, ${user.name}!`;
}

// A class that implements the Employee interface
class Manager implements Employee {
  name: string;
  age: number;
  department: string;
  title: string = "Manager";
  salary: number;
  
  constructor(name: string, age: number, department: string, salary: number) {
    this.name = name;
    this.age = age;
    this.department = department;
    this.salary = salary;
  }
  
  giveRaise(amount: number): void {
    this.salary += amount;
  }
}