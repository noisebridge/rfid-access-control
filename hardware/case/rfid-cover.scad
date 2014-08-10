$fn=96;
case_fn=96;   // make that 8 for faster development, >= 60 for 'pretty'
border_roundness=6;

epsilon=0.05;

// The RFID 522 board size
rfid_w=40;
rfid_h=60;

top_thick =  1.2;  // Thickness of the top shell.
base_thick = 1;    // Thickness of the base-plate, mounted to the wall.
clearance  = 0.5;  // clearance between moving parts.
logo_imprint=0.3;  // depth of the logo imprint.

oval_ratio=rfid_w/rfid_h;

case_height=12;
// inner volume
v_width=rfid_w + 2;
v_height=rfid_h + 8;
v_depth=case_height-base_thick;

top_radius=0.7*rfid_h;  // the longer part.
base_radius=top_radius + 5;
slope_start_fraction=0.6;  // fraction of the height the slope starts.
logo_size=0.75*top_radius;

cleat_angle=35;

cleat_wall_thick = 1.2;

module logo() {
    scale([logo_size,logo_size,1]) linear_extrude(height = logo_imprint + 2*epsilon, convexity = 10)
        translate([-0.8,-0.55,0]) import(file = "Noisebridge-logo.dxf");
}

// For testing.
module pcb_board() {
    color("blue") cube([rfid_w,rfid_h,1.5], center=true);
}

module base_plate() {
    scale([oval_ratio,1,1]) cylinder(r=base_radius - clearance,h=base_thick);
}

module case_inner_volume() {
    scale([oval_ratio,1,1]) {
	cylinder(r=base_radius, h=slope_start_fraction * case_height);
	translate([0,0,slope_start_fraction*case_height - epsilon])
	   cylinder(r1=base_radius, r2=top_radius, h=(1-slope_start_fraction)*case_height, $fn=case_fn);	    
    }
}

// Outer volume above z=0.
module case_outer_volume() {
    minkowski() {
	case_inner_volume();
	translate([0,0,top_thick/2+epsilon]) cube([2*top_thick,2*top_thick,top_thick], center=true);  // slow
    }
}

// top case, hollowed out volume
module top_case() {
    difference() {
	case_outer_volume();
	case_inner_volume();
	translate([0,0,case_height+top_thick - logo_imprint]) logo();
    }
}

module inner_cleat_volume() {
    b=40;  // cut-away block thickness
    // Mmmh, there certainly must be a simpler way to build a parallelogram
    translate([0,2,0]) difference() {
	translate([0, 0, v_depth/2]) cube([v_width, v_height, v_depth], center=true);
	// aligned to the bottom plane
	translate([0,-v_height/2,0]) rotate([-cleat_angle,0,0]) translate([-v_width/2-epsilon,-b,-25]) cube([v_width + 2*epsilon, b, v_depth+50]);

	// aligned to the top plane
	translate([0,v_height/2,v_depth]) rotate([-cleat_angle,0,0]) translate([-v_width/2-epsilon,0,-25]) cube([v_width + 2*epsilon, b, v_depth+50]);
    }
}

module padded_cleat_volume(p=[1,1,1]) {
    minkowski() {
	inner_cleat_volume();
	cube(p, center=true);
    }
}

// The smaller cleat frame, mounted on the base-plate
module inner_cleat_frame() {
    difference() {
	padded_cleat_volume(p=[2*cleat_wall_thick,2*cleat_wall_thick,epsilon]);
	translate([0,0,epsilon]) inner_cleat_volume();
	translate([0,0,-epsilon]) inner_cleat_volume();
    }
}

// The larger cleat frame, mounted on the top.
module outer_cleat_frame() {
    difference() {
	padded_cleat_volume(p=[4*cleat_wall_thick + 2*clearance,4*cleat_wall_thick + 2*clearance, epsilon]);
	translate([0,0,epsilon]) padded_cleat_volume(p=[2*cleat_wall_thick + 2*clearance,2*cleat_wall_thick+2*clearance,epsilon]);
	translate([0,0,-epsilon]) padded_cleat_volume(p=[2*cleat_wall_thick + 2*clearance,2*cleat_wall_thick+2*clearance,epsilon]);
    }
}

module base_assembly() {
    // Some angles of the cleat collide with the inner volume. Give it enough
    // clearance. Since base assembly grows bottom up, we just cut with
    // translation
    intersection() {
	translate([0,0,-clearance]) case_inner_volume();
	union() {
	    color("blue") translate([0,0,base_thick-epsilon]) inner_cleat_frame();
	    base_plate();
	}
    }
}

module case_and_cleat() {
    // the cleat-walls poke through the casing. Clip them with intersection.
    intersection() {
	top_outer_volume();
	union() {
	    intersection() {
		// TODO: something is broken somewhere else, otherwise we
		// should only need 1x clearance here.
		translate([0,0,+3*clearance]) case_inner_volume();
		translate([0,0,case_height - v_depth + epsilon]) color("red") outer_cleat_frame();
	    }
	    top_case();
	}
    }
}

module xray() {
    difference() {
	union() {
	    base_assembly();
	    case_and_cleat();
	}
	translate([0,-50,-epsilon]) cube([100,100,100]);
    }
}

module print() {
    base_assembly();

    // We turn the case-assembly upside down and print next to it.
    translate([2 * oval_ratio * base_radius,0,0]) rotate([0,180,0]) translate([0,0,-case_height - top_thick]) case_and_cleat();
}

print();
//xray();

