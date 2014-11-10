all: standoffs.stl side-holder.stl controller-box.stl

standoffs.stl:
side-holder.stl:
controller-box.stl:

%.gcode : %.stl
	cura $^ -s -o $@

%.stl: %.scad
	openscad -o $@ -d $@.deps $<
