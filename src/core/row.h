#ifndef ROW_H
#define ROW_H

#include <string>
#include "../config.h"

using namespace std;

class Row {
public:
    string data[MAX_ROWS];  // Data for each column in the row

    // Constructor to initialize all data
    Row() {
        for (int i = 0; i < MAX_ROWS; ++i) {
            data[i] = "";
        }
    }

    // Display row data
    void displayRow() const {
        for (int i = 0; i < MAX_ROWS; ++i) {
            if (!data[i].empty()) {
                // cout << data[i] << " ";
            }
        }
        // cout << "\n";
    }
};

#endif
