#include <iostream>
#include "Table.h"

// Constructor to initialize table with a name
Table::Table(const std::string& tableName) : name(tableName), rowCount(0) {}

// Insert a row into the table
bool Table::insertRow(const Row& row) {
    if (rowCount < MAX_ROWS) {
        rows[rowCount++] = row;
        return true;
    } else {
        std::cerr << "Error: Maximum number of rows reached.\n";
        return false;
    }
}

// Display all rows in the table
void Table::displayTable() const {
    std::cout << "Table: " << name << "\n";
    for (int i = 0; i < rowCount; ++i) {
        rows[i].displayRow();
    }
}

// Get the table name
const std::string& Table::getName() const {
    return name;
}

// Get the number of rows in the table
int Table::getRowCount() const {
    return rowCount;
}

// Add a column to the table's schema
void Table::addColumnToSchema(const std::string& name, DataType type, bool isPrimaryKey, bool isUnique, bool isNotNull) {
    schema.addColumn(name, type, isPrimaryKey, isUnique, isNotNull);
}

// Display the schema of the table
void Table::displaySchema() const {
    schema.displaySchema();
}
