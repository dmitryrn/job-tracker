import initSqlJs from "sql.js/dist/sql-asm.js";
import type { Database, SqlJsStatic } from "sql.js";
import { beforeAll, describe, expect, it } from "vitest";
import { inspectSchema, queryTable } from "./database";

let SQL: SqlJsStatic;

beforeAll(async () => {
  SQL = await initSqlJs();
});

function mockDatabase(): Database {
  const database = new SQL.Database();
  database.run(`
    CREATE TABLE companies (id INTEGER PRIMARY KEY, name TEXT NOT NULL);
    CREATE TABLE jobs (id INTEGER PRIMARY KEY, title TEXT NOT NULL, company_id INTEGER);
    INSERT INTO companies (id, name) VALUES (1, 'Studio North'), (2, 'Other Co');
    INSERT INTO jobs (id, title, company_id) VALUES
      (1, 'Analyst', 1),
      (2, 'Engineer', 2),
      (3, 'Designer', 1);
  `);
  return database;
}

describe("SQLite explorer queries", () => {
  it("discovers tables and their columns", () => {
    const database = mockDatabase();

    expect(inspectSchema(database)).toEqual([
      {
        name: "companies",
        kind: "table",
        columns: [
          { name: "id", type: "INTEGER", nullable: true, primaryKey: true },
          { name: "name", type: "TEXT", nullable: false, primaryKey: false },
        ],
      },
      {
        name: "jobs",
        kind: "table",
        columns: [
          { name: "id", type: "INTEGER", nullable: true, primaryKey: true },
          { name: "title", type: "TEXT", nullable: false, primaryKey: false },
          { name: "company_id", type: "INTEGER", nullable: true, primaryKey: false },
        ],
      },
    ]);
    database.close();
  });

  it("supports a filter subquery and sortable columns", () => {
    const database = mockDatabase();

    const rows = queryTable(
      database,
      "jobs",
      "company_id IN (SELECT id FROM companies WHERE name LIKE '%Studio%')",
      { column: "title", direction: "DESC" },
    );

    expect(rows.columns).toEqual(["id", "title", "company_id"]);
    expect(rows.values).toEqual([
      [3, "Designer", 1],
      [1, "Analyst", 1],
    ]);
    database.close();
  });
});
