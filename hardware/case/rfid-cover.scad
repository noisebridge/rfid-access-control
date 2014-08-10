// (c) 2014 h.zeller@acm.org. GNU General Public License 2.0 or higher.
// --
$fn=96;
case_fn=96;
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
v_depth=case_height;

top_radius=0.7*rfid_h;  // the longer part.
base_radius=top_radius + 5;
slope_start_fraction=0.7;  // fraction of the height the slope starts.
logo_size=0.75*top_radius;

cleat_angle=35;
cleat_wall_thick = 1.2;
screw_block_offset=42;

module logo() {
    scale([logo_size,logo_size,1]) linear_extrude(height = logo_imprint + 2*epsilon, convexity = 10)
        translate([-0.8,-0.55,0]) import(file = "Noisebridge-logo.dxf");
}

// For testing.
module pcb_board() {
    color("blue") cube([rfid_w,rfid_h,1.5], center=true);
}

// Screw, standing on its head on the z-plane. Extends a bit on the negative
// z-plane to be able to 'punch' holes.
module countersink_screw(h=15,r=3.2/2,head_r=5.5/2,head_depth=1.6) {
    cylinder(r=r,h=h);
    cylinder(r1=head_r,r2=r,h=head_depth);
    translate([0,0,-1+epsilon]) cylinder(r=head_r,h=1);
}

module mount_screw(r=3.2/2) {
    translate([0,-base_radius-top_thick,slope_start_fraction*case_height/2]) rotate([-90,0,0]) countersink_screw(r=r);
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

// ----
// The cleats are essentially a parallelogram that pushes the case towards
// the back when pulled down. The down-pulling happens with a screw.
// ---

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

module clearance_cleat_volume() {
    padded_cleat_volume(p=[2*cleat_wall_thick + 2*clearance,2*cleat_wall_thick+2*clearance,epsilon]);
}

// The larger cleat frame, mounted on the top.
module outer_cleat_frame() {
    difference() {
	padded_cleat_volume(p=[4*cleat_wall_thick + 2*clearance,4*cleat_wall_thick + 2*clearance, epsilon]);
	translate([0,0,epsilon]) clearance_cleat_volume();
	translate([0,0,-epsilon]) clearance_cleat_volume();
    }
}

// A block that is diagonally split like our cleats. The 'b' parameter is the
// block size, which is centered around x and y. The slit point is at y=0
module diagonal_split_block(b=[1,1,1], left=1) {
    if (left) {
	difference() {
	    translate([-b[0]/2, -b[1]/2, 0]) cube(b);
	    translate([0,0,clearance]) rotate([-cleat_angle, 0, 0]) translate([-50,0,-50]) cube([100,100,100]);
	}
    } else {
	intersection() {
	    translate([-b[0]/2, -b[1]/2, 0]) cube(b);
	    translate([0,0,-clearance]) rotate([-cleat_angle, 0, 0]) translate([-50,0,-50]) cube([100,100,100]);
	}
    }
}

module screw_block(w=15,left=1,padding=0,h=slope_start_fraction * case_height) {
    color("red") translate([0,-screw_block_offset,0]) diagonal_split_block(b=[w + padding,base_radius,h + padding], left=left);
}
    
module base_assembly() {
    // Some angles of the cleat collide with the inner volume. Give it enough
    // clearance. Since base assembly grows bottom up, we just cut with
    // translation
    intersection() {
	translate([0,0,-clearance]) case_inner_volume();
	union() {
	    color("blue") inner_cleat_frame();
	    base_plate();
	}
    }

    // Now the screw holder is actually extending to the outside world
    difference() {
	intersection() {
	    case_outer_volume();
	    screw_block(left=1);
	}
	mount_screw();
    }
}

module case_and_cleat() {
    // The cleat-walls poke through the casing. Clip them with intersection.
    intersection() {
	top_outer_volume();
	intersection() {
	    // Trim fram on the bottom to not interfere with the base.
	    translate([0,0,base_thick+clearance]) case_inner_volume();
	    color("red") outer_cleat_frame();
	}
    }

    // The screw block also needs to be trimmed on the bottom.
    intersection() {
	translate([0,0,base_thick+clearance]) case_inner_volume();
	difference() {
	    // We print upside down, so the block must be the full height
	    // otherwise we have overhang.
	    screw_block(left=0,h=case_height);
	    clearance_cleat_volume();
	    mount_screw(r=1);  // use self-cutting screw for now. Predrill.
	}
    }

    difference() {
	top_case();
	screw_block(left=1, padding=clearance);
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
    translate([-oval_ratio * base_radius,0,0]) base_assembly();

    // We turn the case-assembly upside down and print next to it.
    translate([oval_ratio * base_radius,0,0]) rotate([0,180,0]) translate([0,0,-case_height - top_thick]) case_and_cleat();
}

print();
//xray();

