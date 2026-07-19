// SPDX-License-Identifier: MIT
// Developer diagnostic: resolve every dynamic symbol without starting Xochitl.

#include <dlfcn.h>
#include <stdio.h>

int main(int argc, char **argv)
{
    if (argc != 2) {
        fprintf(stderr, "usage: %s PLUGIN.so\n", argv[0]);
        return 2;
    }
    void *handle = dlopen(argv[1], RTLD_NOW | RTLD_LOCAL);
    if (!handle) {
        fprintf(stderr, "dlopen failed: %s\n", dlerror());
        return 1;
    }
    puts("dlopen=ok");
    dlclose(handle);
    return 0;
}
