
#include <stdio.h>
#include "lcd/lcd-display.h"

int main(int argc, char *argv[]) {
    LCDDisplay display(16);
    if (!display.Init()) {
        fprintf(stderr, "Can't talk with GPIO. Please provide capabilities "
                "or run as root\n");
        return 1;
    }
    display.Print(0, "Hello wörld!");  // UTF-8, yay
}
