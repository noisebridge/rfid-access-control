epsilon=0.1;
acryl_thick=3.3;
side_high=50;
side_thick=8;
side_len=50;
hold_thick=2;
screw_predrill=2;

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
    }
}

// The manual drills are not entirely accurate on the acrylic,
// so adapt to that :)
module left_bracket() {
    difference() {
	translate([0, -(side_len+hold_thick)]) side(right=1);
	// Drill holes.
	translate([-epsilon,-14.96,side_thick-3.627]) rotate([0, 90, 0]) cylinder(r=screw_predrill, h=side_high - 20);
	translate([-epsilon,-36.19,side_thick-4.458]) rotate([0, 90, 0]) cylinder(r=screw_predrill, h=side_high - 20);    
    }
}

module right_bracket() {
    difference() {
	side(right=0);
	translate([-epsilon,13.9,side_thick-5.214]) rotate([0, 90, 0]) cylinder(r=screw_predrill, h=side_high - 20);
	translate([-epsilon,36.8,side_thick-3.703]) rotate([0, 90, 0]) cylinder(r=screw_predrill, h=side_high - 20);        
    }
}

left_bracket();
translate([0, 10]) right_bracket();