// (c) 2014 h.zeller@acm.org. GNU General Public License 2.0 or higher.
// --
$fn=96;
case_fn=96;
border_roundness=6;

epsilon=0.05;

// The RFID 522 board size. Plus some clearance.
rfid_w=40 + 2;
rfid_h=60 + 2;
// from center of board
rfid_hole_r=3.2/2;
rfid_holes = [ [-17,-14], [17,-14], [-12.5, 23], [12.5, 23] ];

top_thick =  1.2;  // Thickness of the top shell.
base_thick = 1;    // Thickness of the base-plate, mounted to the wall.
clearance  = 0.8;  // clearance between moving parts. Depends on printer-Q.
logo_imprint=0.3;  // depth of the logo imprint.

oval_ratio=rfid_w/rfid_h;

case_height=12;    // More precisely: the inner height. Outer is + top_thick.
// inner volume
v_width=rfid_w + 2;
v_height=rfid_h + 8;
v_depth=case_height;

top_radius=0.7*rfid_h;  // the longer part.
base_radius=top_radius + 5;
slope_start_fraction=0.7;  // fraction of the height the slope starts.
logo_size=0.75*top_radius;

cleat_angle=25;
cleat_wall_thick = 1.2; // The thickness of the inner walls of the cleat.
screw_block_offset=42;  // Distance from y-center where the screw-block cut is.
                        // TODO: calculate from other parameters.

// X/Y locations for the wall mount screws. Some manual fiddling involved.
drywall_mount_locations=[ [0, base_radius - 6],
                          [-rfid_w/2+4, -rfid_h/2 + 6],
			  [ rfid_w/2-4, -rfid_h/2 + 6] ];
cable_hole_location = [0,rfid_h/4,-epsilon];

// There is some chrystal on top that needs some space
rfid_mount_height=case_height - 4.6;
rfid_center_offset=[0,3,0];

module logo() {
    scale([logo_size,logo_size,1]) linear_extrude(height = logo_imprint + 2*epsilon, convexity = 10)
        translate([-0.8,-0.55,0]) import(file = "Noisebridge-logo.dxf");
}

// For testing.
module pcb_board() {
    translate(rfid_center_offset) difference() {
	color("blue") translate([0,0,1.5/2]) cube([rfid_w,rfid_h,1.5], center=true);
	for (h = rfid_holes) {
	    translate([h[0], h[1], -epsilon]) cylinder(r=rfid_hole_r,h=2);
	}
    }
}

module pcb_podests() {
    translate(rfid_center_offset) for (h = rfid_holes) {
	translate([h[0], h[1], 0]) cylinder(r=rfid_hole_r,h=case_height);
	translate([h[0], h[1], 0]) cylinder(r=2*rfid_hole_r,h=rfid_mount_height);
    }
}

// Screw, standing on its head on the z-plane. Extends a bit on the negative
// z-plane to be able to 'punch' holes.
module countersunk_screw(h=15,r=3.2/2,head_r=5.5/2,head_depth=1.6) {
    cylinder(r=r,h=h);
    cylinder(r1=head_r,r2=r,h=head_depth);
    translate([0,0,-1+epsilon]) cylinder(r=head_r,h=1);
}

// Some pre-parametrized
module drywall_screw() {
    countersunk_screw(h=45,r=4.2/2,head_r=8.4/2,head_depth=4);
}

module positioned_mount_screw(r=3.2/2) {
    translate([0,-base_radius-top_thick,slope_start_fraction*case_height/2]) rotate([-90,0,0]) countersunk_screw(r=r);
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
    color("red") translate([0,-screw_block_offset,0]) diagonal_split_block(b=[w + 2 * padding,base_radius,h + padding], left=left);
}

module base_screws_podests(raised=4) {
    for (s = drywall_mount_locations) {
	translate([s[0],s[1],0]) cylinder(r=5,h=raised);
    }
}

module base_screws(raised=4) {
    for (s = drywall_mount_locations) {
	translate([s[0],s[1],0])
	     translate([0,0,raised]) rotate([180,0,0]) drywall_screw();
    }
}

module base_assembly() {
    // Some angles of the cleat collide with the inner volume. Give it enough
    // clearance. Since base assembly grows bottom up, we just cut with
    // translation
    intersection() {
	translate([0,0,-clearance]) case_inner_volume();
	difference() {
	    union() {
		color("green") inner_cleat_frame();
		base_plate();
		base_screws_podests();
		pcb_podests();
	    }
	    base_screws();
	    translate(cable_hole_location) cylinder(r=5,h=5);
	}
    }

    // Screw block to mount the top
    // The screw holder is actually extending to the outside world, so we
    // intersect it with the outer volume.
    difference() {
	intersection() {
	    case_outer_volume();
	    screw_block(left=1);
	}
	positioned_mount_screw();
	// Fudging away some sharp corner. Not calculated, needs mnual fiddling.
	translate([-10,-screw_block_offset+2.5,0]) cube([20,5,case_height]);
    }
}

module case_and_cleat() {
    // Cleat walls.
    // They poke through the casing. Clip them with intersection.
    intersection() {
	case_outer_volume();
	intersection() {
	    // Trim fram on the bottom to not interfere with the base.
	    translate([0,0,base_thick+clearance]) case_inner_volume();
	    color("red") outer_cleat_frame();
	}
    }

    // The screw block needs some clearance to the base-plate, so intersect
    // with translated inner volume.
    intersection() {
	translate([0,0,base_thick+clearance]) case_inner_volume();
	difference() {
	    // We print upside down, so the block must be the full height
	    // otherwise we have overhang.
	    screw_block(left=0,h=case_height);
	    clearance_cleat_volume();
	    positioned_mount_screw(r=1);  // Predrill: self-cutting screw for now
	    // Fudging away some sharp corner. manual fiddling.
	    translate([-10,-screw_block_offset-3,0]) cube([20,5,case_height]);
	}
    }

    // The base-plate screw block pokes out of outer case. Cut it out.
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
	    translate([0,0,rfid_mount_height]) pcb_board();
	    %base_screws();
	    %positioned_mount_screw();
	}
	translate([0,-50,-50]) cube([100,100,100]);
	rotate([45,0,0]) translate([-108,top_radius-15,-50]) cube([100,100,100]);
    }
}

module print() {
    translate([-oval_ratio * base_radius,0,0]) base_assembly();

    // We turn the case-assembly upside down and print next to it.
    translate([oval_ratio * base_radius,0,0]) rotate([0,180,0]) translate([0,0,-case_height - top_thick]) case_and_cleat();
}

print();
//xray();
