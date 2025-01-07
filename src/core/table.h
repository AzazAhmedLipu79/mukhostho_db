#ifndef TABLE_H
#define TABLE_H

#include "../config.h"
#include "schema.h"
#include "row.h"

using namespace std;

class Table {
private:
    string name;  // Table name
    Schema schema;    // Table schema
    Row rows[MAX_ROWS];  // Array of rows
    int rowCount;       // Number of rows in the table

public:
    // Constructor
    Table(const string& tableName);

    // Operations
    bool insertRow(const Row& row);  // Insert row into table
    void displayTable() const;       // Display all rows
    const   string& getName() const;  // Get table name
    int getRowCount() const;          // Get number of rows

    // Schema operations
    void addColumnToSchema(const    string& name, DataType type, bool isPrimaryKey = false, bool isUnique = false, bool isNotNull = false);
    void displaySchema() const;  // Display schema
};


#endif