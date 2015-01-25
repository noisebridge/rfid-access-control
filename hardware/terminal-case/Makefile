all : rfid-cover.stl

rfid-cover.stl: Noisebridge-logo.dxf

Noisebridge-logo.dxf:

%.eps: %.svg
	inkscape -E $@ $<

%.dxf: %.eps
	pstoedit -psarg "-r300x300" -dt -f dxf:-polyaslines $< $@

%.stl: %.scad
	openscad -o $@ -d $@.deps $<

clean:
	rm -f rfid-cover.stl Noisebridge-logo.dxf
