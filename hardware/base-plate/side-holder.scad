$fn=8;
epsilon=0.1;
acryl_thick=3;
side_high=50;
side_thick=8;
side_len=50;
hold_thick=2;
screw_predrill=1.8;

module side(right=1) {
    difference() {
	union() {
	    // Base block.
	    cube([side_high, side_len + hold_thick, side_thick]);
	    // Block that covers where the acryl holder is

	    hull() {
		translate([side_high-3*hold_thick, 0, side_thick]) cube([3 * hold_thick, side_len + hold_thick, hold_thick]);
		translate([side_high-6*hold_thick, 0, side_thick-hold_thick]) cube([3 * hold_thick, side_len + hold_thick, hold_thick]);
	    }
	}
	
	// Acryl holder.
	translate([side_high - acryl_thick - hold_thick, right ? hold_thick : -epsilon, -epsilon]) cube([acryl_thick, side_len + 2*epsilon, side_thick+2*epsilon]);

	// Drill holes.
	translate([-epsilon,1*side_len/4,side_thick/2]) rotate([0, 90, 0]) cylinder(r=screw_predrill, h=side_high - 20);
	translate([-epsilon,3*side_len/4,side_thick/2]) rotate([0, 90, 0]) cylinder(r=screw_predrill, h=side_high - 20);    
    }
}

side(right=1);
translate([0, side_len + 5, 0]) side(right=0);