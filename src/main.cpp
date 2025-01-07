#include<iostream>
#include "core/schema.cpp"

using namespace std;

int main() {
    Schema schema;
    string command;

    cout << "Welcome to the In-Memory Database Test Program!" << endl;
    cout << "Available commands:" << endl;
    cout << "  1. create - Create a new schema" << endl;
    cout << "  2. save   - Save the current schema to a file" << endl;
    cout << "  3. load   - Load a schema from a file" << endl;
    cout << "  4. print  - Print the current schema" << endl;
    cout << "  5. exit   - Exit the program" << endl;

    while (true) {
        cout << "\nEnter command: ";
        cin >> command;

        if (command == "create") {
            createSchema(schema);
        } else if (command == "save") {
            saveSchemaToFile(schema);
        } else if (command == "load") {
            string collectionName;
            cout << "Enter collection name to load: ";
            cin >> collectionName;
            if (loadSchemaFromFile(schema, collectionName)) {
                cout << "Schema loaded successfully!" << endl;
            } else {
                cerr << "Failed to load schema." << endl;
            }
        } else if (command == "print") {
            printSchema(schema);
        } else if (command == "exit" || command == "") {
            cout << "Exiting the program. Goodbye!" << endl;
            break;
        } else {
            cerr << "Invalid command. Please try again." << endl;
        }
    }

    return 0;
}
