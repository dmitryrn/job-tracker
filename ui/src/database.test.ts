import initSqlJs from "sql.js/dist/sql-asm.js";
import type { Database, SqlJsStatic } from "sql.js";
import { beforeAll, describe, expect, it } from "vitest";
import { inspectSchema, queryBrowseCompanies, queryBrowseJobs, queryBrowseProviders, queryTable } from "./database";

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

function browseDatabase(): Database {
  const database = new SQL.Database();
  database.run(`
    CREATE TABLE companies (id INTEGER PRIMARY KEY, name TEXT, last_seen_at TEXT);
    CREATE TABLE jobs (
      id INTEGER PRIMARY KEY, source TEXT, source_url TEXT, title TEXT, company_id INTEGER,
      body_text TEXT, location TEXT, workplace TEXT, employment_type TEXT,
      salary_min INTEGER, salary_max INTEGER, posted_at TEXT
    );
    INSERT INTO companies (id, name, last_seen_at) VALUES
      (1, 'Studio North', '2026-08-27T10:00:00Z'),
      (2, 'Other Co', '2026-08-26T10:00:00Z');
    INSERT INTO jobs (id, source, source_url, title, company_id, body_text, location, workplace, employment_type, posted_at)
    VALUES
      (1, 'adzuna', 'https://example.com/1', 'Software Engineer', 1, 'Build useful things.', 'Europe', 'remote', 'full_time', '2026-08-27T09:00:00Z'),
      (2, 'remotive', 'https://example.com/2', 'Product Designer', 2, 'Design useful things.', 'Worldwide', 'remote', 'contract', '2026-08-26T09:00:00Z');
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

  it("queries browse cards and company counts", () => {
    const database = browseDatabase();

    expect(queryBrowseJobs(database, "studio")).toMatchObject([
      {
        id: 1,
        title: "Software Engineer",
        company: "Studio North",
        location: "Europe",
      },
    ]);
    expect(queryBrowseProviders(database)).toEqual(["adzuna", "remotive"]);
    expect(queryBrowseJobs(database, "", "remotive")).toMatchObject([{ id: 2, source: "remotive" }]);
    expect(queryBrowseCompanies(database)).toMatchObject([
      { id: 2, name: "Other Co", jobCount: 1 },
      { id: 1, name: "Studio North", jobCount: 1 },
    ]);
    database.close();
  });
});
