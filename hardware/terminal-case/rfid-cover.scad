// -*- mode: scad; c-basic-offset: 4; indent-tabs-mode: nil; -*-
// (c) 2014 h.zeller@acm.org. GNU General Public License 2.0 or higher.
// (TODO(Henner): now that we know how it should look like: simplify)
// --
$fn=32;
case_fn=320;       // Resolution of the case. Also funky with lo-res 8
border_roundness=6;

case_height=19;    // More precisely: the inner height. Outer is + top_thick.

// Various cable outlets. Negative number to switch off that hole.
cable_to_back_r  = 5;   // radius for cable out the backplane or < 0 for not
cable_to_left_top_r  = -3.5; // radius for cable out of the top, or < 0 for not.
cable_to_right_top_r = 3.5; // radius for cable out of the top, or < 0 for not.

epsilon=0.05;

// The RFID 522 board size. Plus some clearance.
rfid_w=40 + 2;
rfid_h=60 + 2;
// from center of board
rfid_hole_r=2.5/2;
rfid_hole_pos = [ [-17,-14], [17,-14], [-12.5, 23], [12.5, 23] ];
rfid_mount_upside_down=true;  // Upside down mounting brings coil closer to top
rfid_board_thick=1.6;
rfid_board_rest_height=5;  // Height of the lowest board in the sandwich.
rfid_board_register_height=rfid_board_rest_height + 2; // case_height;

top_thick  = 1.2;  // Thickness of the top shell.
base_thick = 1.2;    // Thickness of the base-plate, mounted to the wall.
clearance  = 0.8;  // clearance between moving parts. Depends on printer-Q.
logo_imprint=0.4;  // depth of the logo imprint.

// Screw block below and LED block above
//block_height=case_height/2;
block_height=17.5/2;

oval_ratio=rfid_w/rfid_h;

// inner volume of the cleat-boxed part.
cleat_volume_width  = rfid_w + 1;
cleat_volume_height = rfid_h + 11;  // TODO: calculate that from angle & height

top_radius=0.72*rfid_h;  // the longer part.
base_radius=top_radius + 5;
slope_start_fraction=0.7;  // fraction of the height the slope starts.
logo_size=0.75*top_radius;

cleat_angle=25;
cleat_wall_thick = 1.2; // The thickness of the inner walls of the cleat.
screw_block_offset=base_radius - 5;  // Distance from y-center where the
                                     // diagonal mount-screw cut is.

// X/Y locations for the wall mount screws. Some manual fiddling involved.
drywall_mount_locations=[ [-rfid_w/2+8, rfid_h/2 - 6],
                          [ rfid_w/2-8, rfid_h/2 - 6],
                          [-rfid_w/2+4, -rfid_h/2 + 6],
			  [ rfid_w/2-4, -rfid_h/2 + 6] ];
cable_hole_location = [0,rfid_h/4,-epsilon];

// There is some chrystal on top that needs some space
rfid_mount_height=case_height - (rfid_mount_upside_down ? rfid_board_thick : 4.6) - clearance;
rfid_center_offset=[0,rfid_mount_upside_down?4:2.5,rfid_board_thick/2];

module logo() {
    scale([logo_size,logo_size,1]) linear_extrude(height = logo_imprint + 2*epsilon, convexity = 10)
        translate([-0.82,-0.55,0]) import(file = "Noisebridge-logo.dxf");
}

// For testing.
module pcb_board(board_thick=rfid_board_thick) {
    translate(rfid_center_offset) rotate([0,rfid_mount_upside_down?180:0, 0])
    difference() {
	union() {
	    color("blue") translate([0,0,0]) cube([rfid_w,rfid_h,board_thick], center=true);
	    // Simulate quartz as the highest point.
	    translate([-2,-28,board_thick/2]) color("silver") cube([4,10,3.4]);
	}
	for (h = rfid_hole_pos) {
	    translate([h[0], h[1], -1]) cylinder(r=rfid_hole_r,h=2);
	}
    }
}

module pcb_podests() {
    translate(rfid_center_offset) for (h = rfid_hole_pos) {
	translate([h[0], h[1], 0]) cylinder(r=rfid_hole_r,h=rfid_board_register_height);
	translate([h[0], h[1], 0]) cylinder(r1=2.2*rfid_hole_r,r2=1.5*rfid_hole_r,h=rfid_board_rest_height);
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

module positioned_mount_screw(r=3.3/2) {
    translate([0,-base_radius-top_thick,slope_start_fraction*block_height]) rotate([-90,0,0]) union() {
        countersunk_screw(r=r,h=17);
        // Cutout for the M3
        rotate([0,0,90]) translate([0,0,11]) {
            cylinder(r=6.3/2,h=2.6, $fn=6);
            translate([0,-5.6/2,0]) cube([15,5.6,2.6]);
        }
    }
}

module base_plate() {
    scale([oval_ratio,1,1]) cylinder(r=base_radius - clearance,h=base_thick, $fn=case_fn);
}

module case_inner_volume() {
    scale([oval_ratio,1,1]) {
	cylinder(r=base_radius, h=slope_start_fraction * case_height, $fn=case_fn);
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
module top_case(just_logo=false) {
    // Just printing the logo. Can be used for multi-color print
    if (just_logo) {
        translate([0,0,case_height+top_thick - logo_imprint]) logo();
    } else {
        difference() {
	    case_outer_volume();
	    case_inner_volume();
	    translate([0,0,case_height+top_thick - logo_imprint]) logo();
        }
    }
}

// ----
// The cleats are essentially a parallelogram that pushes the case towards
// the back when pulled down. The down-pulling happens with a screw.
// ---

module inner_cleat_volume(w=cleat_volume_width,
                          h=cleat_volume_height,
		          depth=case_height) {
    b=40;  // cut-away block thickness
    // Mmmh, there certainly must be a simpler way to build a parallelogram...
    translate([0,2,0]) difference() {
	translate([0, 0, depth/2]) cube([w, h, depth], center=true);
	// aligned to the bottom plane
	translate([0,-h/2,0]) rotate([-cleat_angle,0,0]) translate([-w/2-epsilon,-b,-25]) cube([w + 2*epsilon, b, depth+50]);

	// aligned to the top plane
	translate([0,h/2,depth]) rotate([-cleat_angle,0,0]) translate([-w/2-epsilon,0,-40]) cube([w + 2*epsilon, b, depth+50]);
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

// inner cleat volume + inner cleat wall + clearance
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
// This is somewhat hacky. Need to reformulate that concept.
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

// A cable hole that optionally can have a cutout to the bottom.
// (poke_length determines how far outwards the duct goes. Right now we use a
// negative number as we only want to poke through the cleat to connect the LED)
module cable_duct(r=1,cutout=14,xoffset=-18, poke_length=-12) {
    translate([xoffset,0,base_thick + r]) {
        rotate([-90, 0, 0]) {
            cylinder(r=r, h=base_radius+poke_length);
            rotate([0,0,90]) translate([0,-r,0]) cube([cutout, 2*r, base_radius+poke_length]);
        }
    }
}

module cable_drills(cutout=0, widening=0) {
    if (cable_to_back_r > 0)
       translate(cable_hole_location) cylinder(r=cable_to_back_r,h=5);
    if (cable_to_left_top_r > 0)
       cable_duct(xoffset=-18, cable_to_left_top_r + widening, cutout=cutout);
    if (cable_to_right_top_r > 0)
       cable_duct(xoffset=18,cable_to_right_top_r + widening, cutout=cutout);
}

module screw_block(w=19,left=1,padding=0,h=slope_start_fraction * 2*block_height) {
    color("gray") translate([0,-screw_block_offset,0]) diagonal_split_block(b=[w + 2 * padding,base_radius,h + padding], left=left);
}

module led() {
    cylinder(r=3, h=15,$fn=8);
}

module light_block(w=25,padding=0,with_led=true) {
    color("white") translate([-w/2-padding, 40, -epsilon]) union() {
        difference() {
            cube([w+2*padding, 20, block_height+padding]);
            if (with_led) {
                translate([w,0,block_height/2]) rotate([-90,0,60]) led();
            }
        }
    }
}

module base_screws_podests(raised=4) {
    for (s = drywall_mount_locations) {
	translate([s[0],s[1],0]) cylinder(r=5.4,h=raised);
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
	    }
	    base_screws();
	    cable_drills(widening = -0.5); // thus outer shells fit comfortably.
	}
    }

    //pcb_podests(); // allow these to use the full height, e.g. for heatstaking.

    // Screw block to mount the top
    // The screw holder is actually extending to the outside world, so we
    // intersect it with the outer volume.
    difference() {
	intersection() {
	    case_outer_volume();
            union() {
	        screw_block(left=1);
                light_block();
            }
	}
	positioned_mount_screw();
	// Fudging away some sharp corner. Not calculated, needs mnual fiddling.
	translate([-10,-screw_block_offset+2.5,0]) cube([20,5,case_height]);
    }
}

module top_assembly() {
    difference() {
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
	cable_drills(cutout=case_height);
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
	    positioned_mount_screw(r=2.4/2);  // Predrill.
	    // Fudging away some sharp corner. manual fiddling.
	    translate([-10,-screw_block_offset-3,0]) cube([20,5,case_height]);
	}
    }

    // The base-plate screw block pokes out of outer case. Cut it out.
    difference() {
	top_case();
	screw_block(left=1, padding=clearance);
	cable_drills(cutout=case_height);
        light_block(padding=clearance,with_led=false);
    }
}

// Show the whole assembly together.
module show() {
    base_assembly();
    top_assembly();
}

// Like show, but revaling some insight.
module xray() {
    difference() {
	union() {
	    show();
	    translate([0,0,rfid_mount_height]) pcb_board();
	    %base_screws();
	    %positioned_mount_screw();
	}
	translate([0,-60,-60]) cube([120,120,120]);
	rotate([45,0,0]) translate([-108,top_radius-15,-50]) cube([100,100,100]);
    }
}

// The printable form: flat surfaces on the print-bed
module print(print_base=true, print_cover=true,print_logo=false) {
    // This should be printed in transparent
    if (print_base) {
        translate([-oval_ratio * base_radius,0,0]) base_assembly();
    }

    // .. and this in the color the case should be.
    // We turn the case-assembly upside down and print next to it.
    if (print_cover) {
        translate([oval_ratio * base_radius,0,0]) rotate([0,180,0]) translate([0,0,-case_height - top_thick]) top_assembly();
    }
    if (print_logo) {
        translate([oval_ratio * base_radius,0,0]) rotate([0,180,0]) translate([0,0,-case_height - top_thick]) top_case(just_logo=true);
    }
}

show();
//xray();

// Note
//   - print base in transparent
//   - cover: your choice of color
//   - logo: print separately to merge for multi-color imprint.
//print(print_base=true, print_cover=true, print_logo=false);
