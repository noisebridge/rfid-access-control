OBJECTS=lcd-display.o font-data.o gpio.o

CFLAGS=-g -Wall
CXXFLAGS=-g -Wall

libsimplelcd.a: $(OBJECTS)
	ar -rcs $@ $^

font-data.c : font/5x8.bdf font/font2c.awk
	awk -f font/font2c.awk < $< > $@

clean :
	rm -f $(OBJECTS) libsimplelcd.a
