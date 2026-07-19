TEMPLATE = lib
TARGET = paper-agent-image
CONFIG += shared plugin no_plugin_name_prefix c++17

xoviextension.target = xovi.cpp
xoviextension.commands = python3 $$(XOVI_REPO)/util/xovigen.py -o xovi.cpp -H xovi.h paper-agent-image.xovi
xoviextension.depends = paper-agent-image.xovi

QMAKE_EXTRA_TARGETS += xoviextension
PRE_TARGETDEPS += xovi.cpp

QT += quick qml
SOURCES += src/main.cpp xovi.cpp
QMAKE_CXXFLAGS += -fPIC
